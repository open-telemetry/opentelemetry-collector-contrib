# Critical Path E2E Trace: Jaeger Fixture Style

This shape matches Jaeger's own critical-path fixture semantics: the path is not a single vertical chain. The algorithm re-enters the parent after the latest-finishing child and then continues earlier in the trace.

## Mermaid

```mermaid
flowchart TD
    R["root [1,101] CP fragments: [60,101] + [1,20]"]
    A["left [10,50] not on critical path"]
    C["right [20,60] CP fragment: [20,60]"]

    R --> A
    R --> C

    style R fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style C fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style A fill:#f5f5f5,stroke:#888,stroke-width:1px
```

## Expected Result

| Span | On critical path | `exclusive_ns` | `inclusive_ns` |
| --- | --- | ---: | ---: |
| `root` | yes | `60` | `100` |
| `left` | no | omitted | omitted |
| `right` | yes | `40` | `40` |

## Why It Matters

- This is the clearest proof that CRISP/Jaeger semantics allow sibling hops.
- The earlier sibling `left` overlaps the trace but is not selected because the algorithm picks the last-finishing child first, then only considers children whose end is strictly before that child's start.
