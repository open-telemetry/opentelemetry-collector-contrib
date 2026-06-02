# Critical Path E2E Trace: Article-Inspired Tree Transformation

The published article image appears merge-shaped, which is not directly representable as an OTLP span tree because a span can only have one `parentSpanID`.

This test transforms the shape into a valid tree:

- keep the left critical chain as the main lineage
- keep side branches as sibling subtrees
- replace merge nodes with ordinary descendants under the critical lineage
- keep non-critical branches ending before the next critical segment begins

## Mermaid

```mermaid
flowchart TD
    N1["1 [0,200] CP [0,10]"] --> N2["2 [10,200] CP [10,20]"]
    N2 --> N3["3 [20,200] CP [20,30]"]
    N2 --> N6["6 [25,120] not CP"]

    N3 --> N4["4 [30,70] not CP"]
    N3 --> N5["5 [30,200] CP [30,40]"]

    N4 --> N9["9 [40,65] not CP"]
    N9 --> N12["12 [50,60] not CP"]

    N5 --> N7["7 [40,200] CP [40,50]"]
    N5 --> N8["8 [45,90] not CP"]
    N8 --> N10["10 [55,85] not CP"]

    N7 --> N11["11 [50,200] CP [50,60]"]
    N11 --> N13["13 [60,200] CP [60,70]"]
    N13 --> N16["16 [70,200] CP [70,200]"]

    N6 --> N14["14 [35,110] not CP"]
    N6 --> N15["15 [45,100] not CP"]

    style N1 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N2 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N3 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N5 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N7 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N11 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N13 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
    style N16 fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px
```

## Expected Result

| Span | On critical path | `exclusive_ns` | `inclusive_ns` |
| --- | --- | ---: | ---: |
| `1` | yes | `10` | `200` |
| `2` | yes | `10` | `190` |
| `3` | yes | `10` | `180` |
| `5` | yes | `10` | `170` |
| `7` | yes | `10` | `160` |
| `11` | yes | `10` | `150` |
| `13` | yes | `10` | `140` |
| `16` | yes | `130` | `130` |
| all other spans | no | omitted | omitted |

## Why This Transformation

- Preserves the article's “dominant chain plus side branches” intuition.
- Produces a valid tree for OTel/Jaeger semantics.
- Lets us verify the processor against a complex, multi-branch shape without inventing merge semantics the trace model cannot represent.
