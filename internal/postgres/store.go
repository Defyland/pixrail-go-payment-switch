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

type PoolOptions struct {
	MinConns        int32
	MaxConns        int32
	MaxConnLifetime time.Duration
}

func New(ctx context.Context, databaseURL string, options ...PoolOptions) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres pool config: %w", err)
	}
	if len(options) > 0 {
		option := options[0]
		if option.MinConns > 0 {
			config.MinConns = option.MinConns
		}
		if option.MaxConns > 0 {
			config.MaxConns = option.MaxConns
		}
		if option.MaxConnLifetime > 0 {
			config.MaxConnLifetime = option.MaxConnLifetime
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
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

func (s *Store) ClaimSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, claimUntil time.Time) (rail.Transfer, bool, error) {
	if claimToken == "" || !claimUntil.After(time.Now().UTC()) {
		return rail.Transfer{}, false, fmt.Errorf("%w: valid spi claim token and expiry are required", rail.ErrValidation)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rail.Transfer{}, false, err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2 for update`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if err != nil {
		return rail.Transfer{}, false, err
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID != "" {
		return transfer, true, tx.Commit(ctx)
	}
	if transfer.Status != rail.StatusAccepted {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not pending SPI submission", rail.ErrConflict)
	}
	if transfer.SPIClaimedUntil != nil && transfer.SPIClaimedUntil.After(time.Now().UTC()) {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer already claimed for SPI submission", rail.ErrConflict)
	}

	expiresAt := claimUntil.UTC()
	updatedAt := time.Now().UTC()
	transfer.SPIClaimToken = claimToken
	transfer.SPIClaimedUntil = &expiresAt
	transfer.SPISubmissionAttempts++
	transfer.SPILastError = ""
	transfer.UpdatedAt = updatedAt
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set spi_claim_token = $1,
		       spi_claimed_until = $2,
		       spi_submission_attempts = spi_submission_attempts + 1,
		       spi_last_error = '',
		       updated_at = $3
		 where tenant_id = $4 and id = $5`,
		claimToken, expiresAt, updatedAt, tenantID, transferID); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	return transfer, false, tx.Commit(ctx)
}

func (s *Store) RecordSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, message rail.SPIMessage, outbox []events.Event, audit store.AuditRecord) (rail.Transfer, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rail.Transfer{}, false, err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2 for update`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if err != nil {
		return rail.Transfer{}, false, err
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID == message.MessageID {
		return transfer, true, tx.Commit(ctx)
	}
	if transfer.Status != rail.StatusAccepted {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not pending SPI submission", rail.ErrConflict)
	}
	if transfer.SPIClaimToken == "" || transfer.SPIClaimToken != claimToken {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not claimed by this SPI worker", rail.ErrConflict)
	}
	if message.MessageID == "" || message.EndToEndID == "" {
		return rail.Transfer{}, false, fmt.Errorf("%w: spi identifiers are required", rail.ErrValidation)
	}
	transfer.Status = rail.StatusApproved
	transfer.SPIMessageID = message.MessageID
	transfer.EndToEndID = message.EndToEndID
	transfer.SPIClaimToken = ""
	transfer.SPIClaimedUntil = nil
	transfer.SPILastError = ""
	transfer.UpdatedAt = message.SubmittedAt.UTC()
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set status = $1,
		       spi_message_id = $2,
		       end_to_end_id = $3,
		       spi_claim_token = '',
		       spi_claimed_until = null,
		       spi_last_error = '',
		       updated_at = $4
		 where tenant_id = $5 and id = $6`,
		transfer.Status, transfer.SPIMessageID, transfer.EndToEndID, transfer.UpdatedAt, tenantID, transferID); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	for _, event := range outbox {
		if err := insertOutbox(ctx, tx, event); err != nil {
			return rail.Transfer{}, false, translatePgError(err)
		}
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	return transfer, false, tx.Commit(ctx)
}

func (s *Store) ReleaseSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, lastError string, retryAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2 for update`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.ErrNotFound
	}
	if err != nil {
		return err
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID != "" {
		return tx.Commit(ctx)
	}
	if transfer.SPIClaimToken == "" || transfer.SPIClaimToken != claimToken {
		return fmt.Errorf("%w: transfer is not claimed by this SPI worker", rail.ErrConflict)
	}
	availableAt := retryAt.UTC()
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set spi_claim_token = '',
		       spi_claimed_until = $1,
		       spi_last_error = $2,
		       updated_at = $3
		 where tenant_id = $4 and id = $5`,
		availableAt, lastError, time.Now().UTC(), tenantID, transferID); err != nil {
		return translatePgError(err)
	}
	return tx.Commit(ctx)
}

func (s *Store) RecordReviewDecision(ctx context.Context, tenantID string, transferID string, status rail.TransferStatus, reason string, outbox []events.Event, audit store.AuditRecord) (rail.Transfer, error) {
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
	if transfer.Status != rail.StatusReview {
		return rail.Transfer{}, fmt.Errorf("%w: transfer is not waiting for review", rail.ErrConflict)
	}
	switch status {
	case rail.StatusAccepted, rail.StatusBlocked:
	default:
		return rail.Transfer{}, fmt.Errorf("%w: review can only accept or block", rail.ErrValidation)
	}
	transfer.Status = status
	if reason != "" {
		transfer.DecisionReason = reason
	}
	transfer.UpdatedAt = audit.CreatedAt.UTC()
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set status = $1, decision_reason = $2, updated_at = $3
		 where tenant_id = $4 and id = $5`,
		transfer.Status, transfer.DecisionReason, transfer.UpdatedAt, tenantID, transferID); err != nil {
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

func (s *Store) UpdateSettlement(ctx context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit store.AuditRecord) (rail.Transfer, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rail.Transfer{}, false, err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, transferSelectSQL()+` where tenant_id = $1 and id = $2 for update`, tenantID, transferID)
	transfer, err := scanTransfer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if err != nil {
		return rail.Transfer{}, false, err
	}
	if transfer.SPIMessageID == "" || transfer.SPIMessageID != callback.SPIMessageID {
		return rail.Transfer{}, false, fmt.Errorf("%w: spi_message_id mismatch", rail.ErrConflict)
	}
	callbackHash := callback.CallbackHash
	if callbackHash == "" {
		callbackHash = callback.Fingerprint()
	}
	var existingHash string
	err = tx.QueryRow(ctx, `
		select callback_hash
		  from processed_spi_callbacks
		 where tenant_id = $1 and spi_message_id = $2`,
		tenantID, callback.SPIMessageID).Scan(&existingHash)
	if err == nil {
		if existingHash == callbackHash {
			return transfer, true, tx.Commit(ctx)
		}
		return rail.Transfer{}, false, fmt.Errorf("%w: conflicting settlement callback for terminal transfer", rail.ErrConflict)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return rail.Transfer{}, false, err
	}
	if transfer.Status.Terminal() {
		return rail.Transfer{}, false, fmt.Errorf("%w: terminal transfer has no matching callback hash", rail.ErrConflict)
	}
	if transfer.Status != rail.StatusApproved {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not approved for settlement callback", rail.ErrConflict)
	}
	switch callback.Status {
	case rail.SettlementAccepted:
		transfer.Status = rail.StatusSettled
	case rail.SettlementRejected:
		transfer.Status = rail.StatusRejected
	default:
		return rail.Transfer{}, false, fmt.Errorf("%w: unsupported settlement status", rail.ErrValidation)
	}
	transfer.SettlementCode = callback.Code
	transfer.UpdatedAt = callback.ReceivedAt.UTC()
	if _, err := tx.Exec(ctx, `
		update pix_transfers
		   set status = $1, settlement_code = $2, updated_at = $3
		 where tenant_id = $4 and id = $5`,
		transfer.Status, transfer.SettlementCode, transfer.UpdatedAt, tenantID, transferID); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	for _, event := range outbox {
		if err := insertOutbox(ctx, tx, event); err != nil {
			return rail.Transfer{}, false, translatePgError(err)
		}
	}
	if err := insertAudit(ctx, tx, audit); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	if _, err := tx.Exec(ctx, `
		insert into processed_spi_callbacks (tenant_id, spi_message_id, callback_hash, processed_at)
		values ($1, $2, $3, $4)`,
		tenantID, callback.SPIMessageID, callbackHash, callback.ReceivedAt.UTC()); err != nil {
		return rail.Transfer{}, false, translatePgError(err)
	}
	return transfer, false, tx.Commit(ctx)
}

func (s *Store) Outbox(ctx context.Context) ([]events.OutboxRecord, error) {
	rows, err := s.pool.Query(ctx, outboxSelectSQL()+` order by sequence asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanOutboxRows(rows)
	if err != nil {
		return nil, err
	}
	return records, rows.Err()
}

func (s *Store) Audit(ctx context.Context) ([]store.AuditRecord, error) {
	rows, err := s.pool.Query(ctx, `
		select tenant_id, account_id, pix_transfer_id, action, correlation_id, metadata, created_at
		  from audit_records
		 order by id asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []store.AuditRecord
	for rows.Next() {
		var record store.AuditRecord
		var metadata []byte
		if err := rows.Scan(&record.TenantID, &record.AccountID, &record.TransferID, &record.Action, &record.CorrelationID, &metadata, &record.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadata, &record.Metadata); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) PendingOutbox(ctx context.Context, limit int) ([]events.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, outboxSelectSQL()+`
		where published = false
		  and available_at <= now()
		  and (claimed_until is null or claimed_until <= now())
		order by sequence asc
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanOutboxRows(rows)
	if err != nil {
		return nil, err
	}
	return records, rows.Err()
}

func (s *Store) ClaimPendingOutbox(ctx context.Context, limit int, claimToken string, claimUntil time.Time) ([]events.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if claimToken == "" || !claimUntil.After(time.Now().UTC()) {
		return nil, fmt.Errorf("%w: valid outbox claim token and expiry are required", rail.ErrValidation)
	}
	rows, err := s.pool.Query(ctx, `
		with next_records as (
			select sequence
			  from payment_outbox
			 where published = false
			   and available_at <= now()
			   and (claimed_until is null or claimed_until <= now())
			 order by sequence asc
			 limit $1
			 for update skip locked
		)
		update payment_outbox p
		   set claim_token = $2,
		       claimed_until = $3
		  from next_records n
		 where p.sequence = n.sequence
		returning p.sequence, p.event_id, p.event_type, p.schema_version, p.occurred_at, p.producer,
		          p.tenant_id, p.account_id, p.pix_transfer_id, p.correlation_id, p.payload,
		          p.published, p.attempts, p.last_error, p.available_at, p.claim_token,
		          p.claimed_until, p.dispatched_at`,
		limit, claimToken, claimUntil.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanOutboxRows(rows)
	if err != nil {
		return nil, err
	}
	return records, rows.Err()
}

func (s *Store) PendingSPISubmissions(ctx context.Context, limit int) ([]rail.Transfer, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, transferSelectSQL()+`
		where status = $1
		  and coalesce(spi_message_id, '') = ''
		  and (spi_claimed_until is null or spi_claimed_until <= now())
		order by created_at asc
		limit $2`, rail.StatusAccepted, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []rail.Transfer
	for rows.Next() {
		transfer, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, transfer)
	}
	return transfers, rows.Err()
}

func (s *Store) MarkOutboxPublished(ctx context.Context, sequence int64, claimToken string, dispatchedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update payment_outbox
		   set published = true,
		       dispatched_at = $3,
		       last_error = '',
		       claim_token = '',
		       claimed_until = null
		 where sequence = $1 and claim_token = $2`, sequence, claimToken, dispatchedAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: outbox record is not claimed by this worker", rail.ErrConflict)
	}
	return nil
}

func (s *Store) MarkOutboxFailed(ctx context.Context, sequence int64, claimToken string, lastError string, retryAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update payment_outbox
		   set attempts = attempts + 1,
		       last_error = $3,
		       available_at = $4,
		       claim_token = '',
		       claimed_until = null
		 where sequence = $1 and claim_token = $2`, sequence, claimToken, lastError, retryAt.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: outbox record is not claimed by this worker", rail.ErrConflict)
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
		&transfer.RequestHash,
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
		&transfer.SPIClaimToken,
		&transfer.SPIClaimedUntil,
		&transfer.SPISubmissionAttempts,
		&transfer.SPILastError,
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
		select id, tenant_id, account_id, idempotency_key, request_hash, correlation_id, coalesce(end_to_end_id, ''),
		       amount_cents, currency, receiver_key, receiver_key_type, receiver_name, receiver_bank,
		       receiver_risk, fraud_score, fraud_rules, status, decision_reason, coalesce(spi_message_id, ''),
		       spi_claim_token, spi_claimed_until, spi_submission_attempts, spi_last_error,
		       settlement_code, created_at, updated_at
		  from pix_transfers`
}

func insertTransfer(ctx context.Context, tx pgx.Tx, transfer rail.Transfer) error {
	fraudRules, err := json.Marshal(transfer.FraudRules)
	if err != nil {
		return err
	}
	var spiClaimedUntil any
	if transfer.SPIClaimedUntil != nil {
		spiClaimedUntil = transfer.SPIClaimedUntil.UTC()
	}
	_, err = tx.Exec(ctx, `
		insert into pix_transfers (
			id, tenant_id, account_id, idempotency_key, request_hash, correlation_id, end_to_end_id,
			amount_cents, currency, receiver_key, receiver_key_type, receiver_name, receiver_bank,
			receiver_risk, fraud_score, fraud_rules, status, decision_reason, spi_message_id,
			spi_claim_token, spi_claimed_until, spi_submission_attempts, spi_last_error,
			settlement_code, created_at, updated_at
		) values (
			$1, $2, $3, $4, $5, $6, nullif($7, ''),
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, nullif($19, ''),
			$20, $21, $22, $23,
			$24, $25, $26
		)`,
		transfer.ID, transfer.TenantID, transfer.AccountID, transfer.IdempotencyKey, transfer.RequestHash, transfer.CorrelationID, transfer.EndToEndID,
		transfer.AmountCents, transfer.Currency, transfer.ReceiverKey, transfer.ReceiverKeyType, transfer.ReceiverName, transfer.ReceiverBank,
		transfer.ReceiverRisk, transfer.FraudScore, fraudRules, transfer.Status, transfer.DecisionReason, transfer.SPIMessageID,
		transfer.SPIClaimToken, spiClaimedUntil, transfer.SPISubmissionAttempts, transfer.SPILastError,
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
		       pix_transfer_id, correlation_id, payload, published, attempts, last_error, available_at,
		       claim_token, claimed_until, dispatched_at
		  from payment_outbox`
}

func scanOutboxRows(rows pgx.Rows) ([]events.OutboxRecord, error) {
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
			&record.ClaimToken,
			&record.ClaimedUntil,
			&record.DispatchedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
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
