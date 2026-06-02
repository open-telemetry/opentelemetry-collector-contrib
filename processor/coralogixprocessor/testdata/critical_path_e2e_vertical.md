# Critical Path E2E Trace: Vertical Chain

This scenario keeps one dominant branch active all the way to trace completion, so the critical path is a single vertical chain.

## Mermaid

```mermaid
flowchart TD
    R["root [0,150] CP fragment: [0,10]"]
    A["branch-a [10,150] CP fragment: [10,30]"]
    ADB["branch-a-db [30,150] CP fragment: [30,70]"]
    AIO["branch-a-io [70,150] CP fragment: [70,150]"]
    B["branch-b [20,80] not on critical path"]
    C["branch-c [90,120] not on critical path"]

    R --> A
    A --> ADB
    ADB --> AIO
    R --> B
    R --> C

    style R fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style A fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style ADB fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style AIO fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style B fill:#f5f5f5,stroke:#888,stroke-width:1px
    style C fill:#f5f5f5,stroke:#888,stroke-width:1px
```

## Expected Result

| Span | On critical path | `exclusive_ns` | `inclusive_ns` |
| --- | --- | ---: | ---: |
| `root` | yes | `10` | `150` |
| `branch-a` | yes | `20` | `140` |
| `branch-a-db` | yes | `40` | `120` |
| `branch-a-io` | yes | `80` | `80` |
| `branch-b` | no | omitted | omitted |
| `branch-c` | no | omitted | omitted |

## Notes

- This shape matches the intuition of one uninterrupted latency-determining chain.
- Sibling branches exist, but they end before the active backward cursor reaches them, so they never become critical-path fragments.
