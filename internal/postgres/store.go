package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Health(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) FindByIdempotency(ctx context.Context, tenantID, key string) (rail.Transfer, bool, error) {
	row := s.pool.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and idempotency_key = $2`, tenantID, key)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, false, nil
	}
	if err != nil {
		return rail.Transfer{}, false, err
	}
	return transfer, true, nil
}

func (s *Store) InsertTransfer(ctx context.Context, transfer rail.Transfer, outbox []events.Event, audit []store.AuditRecord) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	if err := insertTransfer(ctx, tx, transfer); err != nil {
		return translatePgError(err)
	}
	for _, event := range outbox {
		if err := insertOutbox(ctx, tx, event); err != nil {
			return translatePgError(err)
		}
	}
	for _, record := range audit {
		if err := insertAudit(ctx, tx, record); err != nil {
			return translatePgError(err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetTransfer(ctx context.Context, tenantID, transferID string) (rail.Transfer, error) {
	row := s.pool.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, rail.ErrNotFound
	}
	return transfer, err
}

func (s *Store) UpdateSettlement(ctx context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit store.AuditRecord) (rail.Transfer, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rail.Transfer{}, err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2 for update`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, rail.ErrNotFound
	}
	if err != nil {
		return rail.Transfer{}, err
	}
	if transfer.SPIMessageID == "" || transfer.SPIMessageID != callback.SPIMessageID {
		return rail.Transfer{}, fmt.Errorf("%w: spi_message_id mismatch", rail.ErrConflict)
	}
	if transfer.Status.Terminal() {
		return transfer, tx.Commit(ctx)
	}
	switch callback.Status {
	case rail.SettlementAccepted:
		transfer.Status = rail.StatusSettled
	case rail.SettlementRejected:
		transfer.Status = rail.StatusRejected
	default:
		return rail.Transfer{}, fmt.Errorf("%w: unsupported settlement status", rail.ErrValidation)
	}
	transfer.SettlementCode = callback.Code
	transfer.UpdatedAt = callback.ReceivedAt.UTC()
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set status = $1, settlement_code = $2, updated_at = $3
		 where tenant_id = $4 and id = $5`,
		transfer.Status, transfer.SettlementCode, transfer.UpdatedAt, tenantID, transferID); err != nil {
		return rail.Transfer{}, translatePgError(err)
	}
	for _, event := range outbox {
		if err := insertOutbox(ctx, tx, event); err != nil {
			return rail.Transfer{}, translatePgError(err)
		}
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return rail.Transfer{}, translatePgError(err)
	}
	return transfer, tx.Commit(ctx)
}

func (s *Store) Outbox(ctx context.Context) []events.OutboxRecord {
	rows, err := s.pool.Query(ctx, outboxSelectSQL()+` order by sequence asc`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanOutboxRows(rows)
}

func (s *Store) Audit(ctx context.Context) []store.AuditRecord {
	rows, err := s.pool.Query(ctx, `
		select tenant_id, account_id, pix_transfer_id, action, correlation_id, metadata, created_at
		  from audit_records
		 order by id asc`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []store.AuditRecord
	for rows.Next() {
		var record store.AuditRecord
		var metadata []byte
		if err := rows.Scan(&record.TenantID, &record.AccountID, &record.TransferID, &record.Action, &record.CorrelationID, &metadata, &record.CreatedAt); err != nil {
			return nil
		}
		_ = json.Unmarshal(metadata, &record.Metadata)
		records = append(records, record)
	}
	return records
}

func (s *Store) PendingOutbox(ctx context.Context, limit int) ([]events.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, outboxSelectSQL()+`
		where published = false and available_at <= now()
		order by sequence asc
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOutboxRows(rows), rows.Err()
}

func (s *Store) MarkOutboxPublished(ctx context.Context, sequence int64, dispatchedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update payment_outbox
		   set published = true, dispatched_at = $2, last_error = ''
		 where sequence = $1`, sequence, dispatchedAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return rail.ErrNotFound
	}
	return nil
}

func (s *Store) MarkOutboxFailed(ctx context.Context, sequence int64, lastError string, retryAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update payment_outbox
		   set attempts = attempts + 1, last_error = $2, available_at = $3
		 where sequence = $1`, sequence, lastError, retryAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return rail.ErrNotFound
	}
	return nil
}

type transferScanner interface {
	Scan(dest ...any) error
}

func scanTransfer(row transferScanner) (rail.Transfer, error) {
	var transfer rail.Transfer
	var fraudRules []byte
	err := row.Scan(
		&transfer.ID,
		&transfer.TenantID,
		&transfer.AccountID,
		&transfer.IdempotencyKey,
		&transfer.CorrelationID,
		&transfer.EndToEndID,
		&transfer.AmountCents,
		&transfer.Currency,
		&transfer.ReceiverKey,
		&transfer.ReceiverKeyType,
		&transfer.ReceiverName,
		&transfer.ReceiverBank,
		&transfer.ReceiverRisk,
		&transfer.FraudScore,
		&fraudRules,
		&transfer.Status,
		&transfer.DecisionReason,
		&transfer.SPIMessageID,
		&transfer.SettlementCode,
		&transfer.CreatedAt,
		&transfer.UpdatedAt,
	)
	if err != nil {
		return rail.Transfer{}, err
	}
	if err := json.Unmarshal(fraudRules, &transfer.FraudRules); err != nil {
		return rail.Transfer{}, err
	}
	return transfer, nil
}

func transferSelectSQL() string {
	return `
		select id, tenant_id, account_id, idempotency_key, correlation_id, coalesce(end_to_end_id, ''),
		       amount_cents, currency, receiver_key, receiver_key_type, receiver_name, receiver_bank,
		       receiver_risk, fraud_score, fraud_rules, status, decision_reason, coalesce(spi_message_id, ''),
		       settlement_code, created_at, updated_at
		  from pix_transfers`
}

func insertTransfer(ctx context.Context, tx pgx.Tx, transfer rail.Transfer) error {
	fraudRules, err := json.Marshal(transfer.FraudRules)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into pix_transfers (
			id, tenant_id, account_id, idempotency_key, correlation_id, end_to_end_id,
			amount_cents, currency, receiver_key, receiver_key_type, receiver_name, receiver_bank,
			receiver_risk, fraud_score, fraud_rules, status, decision_reason, spi_message_id,
			settlement_code, created_at, updated_at
		) values (
			$1, $2, $3, $4, $5, nullif($6, ''),
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, nullif($18, ''),
			$19, $20, $21
		)`,
		transfer.ID, transfer.TenantID, transfer.AccountID, transfer.IdempotencyKey, transfer.CorrelationID, transfer.EndToEndID,
		transfer.AmountCents, transfer.Currency, transfer.ReceiverKey, transfer.ReceiverKeyType, transfer.ReceiverName, transfer.ReceiverBank,
		transfer.ReceiverRisk, transfer.FraudScore, fraudRules, transfer.Status, transfer.DecisionReason, transfer.SPIMessageID,
		transfer.SettlementCode, transfer.CreatedAt.UTC(), transfer.UpdatedAt.UTC(),
	)
	return err
}

func insertOutbox(ctx context.Context, tx pgx.Tx, event events.Event) error {
	_, err := tx.Exec(ctx, `
		insert into payment_outbox (
			event_id, event_type, schema_version, occurred_at, producer, tenant_id, account_id,
			pix_transfer_id, correlation_id, payload, available_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		event.EventID, event.EventType, event.SchemaVersion, event.OccurredAt.UTC(), event.Producer, event.TenantID, event.AccountID,
		event.TransferID, event.CorrelationID, event.Payload, event.OccurredAt.UTC(),
	)
	return err
}

func insertAudit(ctx context.Context, tx pgx.Tx, record store.AuditRecord) error {
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into audit_records (
			tenant_id, account_id, pix_transfer_id, action, correlation_id, metadata, created_at
		) values ($1, $2, $3, $4, $5, $6, $7)`,
		record.TenantID, record.AccountID, record.TransferID, record.Action, record.CorrelationID, metadata, record.CreatedAt.UTC(),
	)
	return err
}

func outboxSelectSQL() string {
	return `
		select sequence, event_id, event_type, schema_version, occurred_at, producer, tenant_id, account_id,
		       pix_transfer_id, correlation_id, payload, published, attempts, last_error, available_at, dispatched_at
		  from payment_outbox`
}

func scanOutboxRows(rows pgx.Rows) []events.OutboxRecord {
	var records []events.OutboxRecord
	for rows.Next() {
		var record events.OutboxRecord
		err := rows.Scan(
			&record.Sequence,
			&record.Event.EventID,
			&record.Event.EventType,
			&record.Event.SchemaVersion,
			&record.Event.OccurredAt,
			&record.Event.Producer,
			&record.Event.TenantID,
			&record.Event.AccountID,
			&record.Event.TransferID,
			&record.Event.CorrelationID,
			&record.Event.Payload,
			&record.Published,
			&record.Attempts,
			&record.LastError,
			&record.AvailableAt,
			&record.DispatchedAt,
		)
		if err != nil {
			return nil
		}
		records = append(records, record)
	}
	return records
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func translatePgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s", rail.ErrConflict, pgErr.ConstraintName)
	}
	return err
}
