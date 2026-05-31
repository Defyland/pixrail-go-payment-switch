alter table pix_transfers
  add column if not exists request_hash text not null default 'legacy-unfingerprinted';

alter table pix_transfers
  drop constraint if exists pix_transfers_status_check;

alter table pix_transfers
  add constraint pix_transfers_status_check
  check (status in ('accepted', 'approved', 'blocked', 'review', 'settled', 'rejected'));

create index if not exists pix_transfers_pending_spi_idx
  on pix_transfers (created_at asc)
  where status = 'accepted' and spi_message_id is null;

create table if not exists processed_spi_callbacks (
  tenant_id text not null,
  spi_message_id text not null,
  callback_hash text not null,
  processed_at timestamptz not null,
  primary key (tenant_id, spi_message_id, callback_hash)
);
