# State Machines

## Transfer state

```mermaid
stateDiagram-v2
  [*] --> approved: DICT resolved + fraud approved
  [*] --> review: fraud review threshold
  [*] --> blocked: fraud block threshold
  approved --> settled: SPI accepted callback
  approved --> rejected: SPI rejected callback
  blocked --> [*]
  review --> [*]
  settled --> [*]
  rejected --> [*]
```

## Transition rules

- `blocked` is terminal in PixRail.
- `review` does not create a SPI message in the current MVP.
- `settled` and `rejected` are terminal.
- Duplicate callbacks for terminal transfers replay the terminal state.
- A callback with a mismatched SPI message ID is a conflict.
