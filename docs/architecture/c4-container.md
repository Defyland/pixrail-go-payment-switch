# C4 Container

```mermaid
flowchart TB
  API["Go HTTP API"] --> Switch["Payment switch service"]
  Switch --> Store["Store port"]
  Store --> Memory["Memory adapter"]
  Store --> Postgres["PostgreSQL adapter"]
  Switch --> Dict["DICT resolver port"]
  Switch --> Fraud["Fraud engine"]
  Switch --> SPI["SPI client port"]
  Worker["SPI worker"] --> Switch
  Worker --> Store
  Relay["Outbox relay"] --> Store
  Relay --> Broker["Broker publisher port"]
  API --> Obs["Metrics, logs, traces"]
  Worker --> Obs
```

The API, SPI worker, and relay share the same switch/store contracts. The Compose runtime starts the API and SPI worker as separate processes; a broker-backed relay remains an adapter milestone because no external broker is required for the local challenge runtime.
