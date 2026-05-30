# Data Classification

| Data | Classification | Handling |
| --- | --- | --- |
| API keys | secret | environment variables, never logged |
| Tenant ID | internal | logged for operations only when needed |
| Account ID | internal sensitive | used as partition key, avoid public exposure beyond tenant |
| Receiver key | sensitive payment data | validate input; avoid logging full value |
| Fraud rules and score | internal risk data | audit and protect from public exposure |
| SPI message ID | internal payment-network evidence | safe for operations logs with correlation ID |
| Event payloads | internal integration data | consumers must enforce tenant boundaries |

The local MVP does not encrypt records at the application layer. Production relies on database encryption at rest, TLS in transit, and strict secret management.
