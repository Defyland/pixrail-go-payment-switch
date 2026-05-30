# Payment Rail Event Drift

Use this runbook when downstream systems reject PixRail payment-rail events.

## Triage

- Identify the event schema under `docs/events/`.
- Confirm `account_id` partitioning, `pix_transfer_id`, `event_id`, and `correlation_id`.
- Check DICT, fraud, and SPI decision logs for the transfer.
- Verify SettleFlow did not treat a PixRail event as ledger truth without its own command handling.

## Recovery

- Restore a backward-compatible event payload.
- Replay events by account partition to preserve ordering.
- Keep duplicate SPI callbacks deduplicated by SPI message ID.
- Add a new schema version for breaking changes.
