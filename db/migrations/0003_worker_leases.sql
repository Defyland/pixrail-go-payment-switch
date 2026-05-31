alter table pix_transfers
  add column if not exists spi_claim_token text not null default '',
  add column if not exists spi_claimed_until timestamptz,
  add column if not exists spi_submission_attempts integer not null default 0,
  add column if not exists spi_last_error text not null default '';

create index if not exists pix_transfers_spi_claim_idx
  on pix_transfers (status, spi_claimed_until, created_at asc)
  where spi_message_id is null;

alter table payment_outbox
  add column if not exists claim_token text not null default '',
  add column if not exists claimed_until timestamptz;

create index if not exists payment_outbox_claim_idx
  on payment_outbox (published, claimed_until, available_at, sequence);
