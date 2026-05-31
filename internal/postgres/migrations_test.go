package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationDocumentsProductionConstraints(t *testing.T) {
	raw, err := os.ReadFile("../../db/migrations/0001_pixrail_core.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(raw))
	required := []string{
		"create table if not exists pix_transfers",
		"request_hash text not null",
		"unique (tenant_id, idempotency_key)",
		"spi_message_id text unique",
		"pix_transfers_pending_spi_idx",
		"create table if not exists payment_outbox",
		"event_id text not null unique",
		"payment_outbox_pending_idx",
		"where published = false",
		"create table if not exists audit_records",
		"create table if not exists processed_spi_callbacks",
		"primary key (tenant_id, spi_message_id, callback_hash)",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
