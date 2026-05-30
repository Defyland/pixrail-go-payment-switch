create table if not exists pix_transfers (
  id text primary key,
  tenant_id text not null,
  account_id text not null,
  idempotency_key text not null,
  correlation_id text not null,
  end_to_end_id text unique,
  amount_cents bigint not null check (amount_cents > 0),
  currency text not null check (currency = 'BRL'),
  receiver_key text not null,
  receiver_key_type text not null,
  receiver_name text not null,
  receiver_bank text not null,
  receiver_risk integer not null,
  fraud_score integer not null,
  fraud_rules jsonb not null default '[]'::jsonb,
  status text not null check (status in ('approved', 'blocked', 'review', 'settled', 'rejected')),
  decision_reason text not null,
  spi_message_id text unique,
  settlement_code text not null default '',
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique (tenant_id, idempotency_key)
);

create index if not exists pix_transfers_tenant_account_created_idx
  on pix_transfers (tenant_id, account_id, created_at desc);

create table if not exists payment_outbox (
  sequence bigserial primary key,
  event_id text not null unique,
  event_type text not null,
  schema_version text not null,
  occurred_at timestamptz not null,
  producer text not null,
  tenant_id text not null,
  account_id text not null,
  pix_transfer_id text not null references pix_transfers(id),
  correlation_id text not null,
  payload jsonb not null,
  published boolean not null default false,
  attempts integer not null default 0,
  last_error text not null default '',
  available_at timestamptz not null,
  dispatched_at timestamptz
);

create index if not exists payment_outbox_pending_idx
  on payment_outbox (available_at, sequence)
  where published = false;

create table if not exists audit_records (
  id bigserial primary key,
  tenant_id text not null,
  account_id text not null,
  pix_transfer_id text not null references pix_transfers(id),
  action text not null,
  correlation_id text not null,
  metadata jsonb not null,
  created_at timestamptz not null
);

create index if not exists audit_records_transfer_idx
  on audit_records (tenant_id, pix_transfer_id, created_at desc);

create table if not exists processed_spi_callbacks (
  tenant_id text not null,
  spi_message_id text not null,
  callback_hash text not null,
  processed_at timestamptz not null,
  primary key (tenant_id, spi_message_id, callback_hash)
);
