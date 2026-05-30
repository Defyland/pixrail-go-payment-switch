# Pricing or Plans

PixRail is a portfolio backend project, not a commercial SaaS. The product can still be reasoned about as an internal platform component with usage tiers:

| Plan | Target user | Constraint |
| --- | --- | --- |
| Sandbox | fintech developers | in-memory adapters, local API key, fake DICT/SPI |
| Partner staging | integration teams | durable storage, broker relay, signed callbacks |
| Production rail | fintech platform | provider certification, SLOs, Redis rate limits, PostgreSQL HA |

This framing matters because the MVP should not carry the cost of the production rail before the payment-switch domain is proven.
