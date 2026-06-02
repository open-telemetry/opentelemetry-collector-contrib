# Critical Path E2E Scenarios

This file documents the 21 processor-level end-to-end critical-path scenarios in [e2e_critical_path_test.go](/Users/israel.blancas/projects/contrib9/processor/coralogixprocessor/e2e_critical_path_test.go).

Highlighted nodes are expected to be on the critical path.

## 1. `jaeger_fixture_sibling_hop`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R1["root"]:::critical --> A1["left"]:::normal
    R1 --> C1["right"]:::critical
```

Expected:
- `root`: `exclusive_ns=60`, `inclusive_ns=100`
- `right`: `exclusive_ns=40`, `inclusive_ns=40`

## 2. `vertical_chain`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R2["root"]:::critical --> A2["branch-a"]:::critical
    A2 --> DB2["branch-a-db"]:::critical
    DB2 --> IO2["branch-a-io"]:::critical
    R2 --> B2["branch-b"]:::normal
    R2 --> C2["branch-c"]:::normal
```

Expected:
- `root`: `10`, `150`
- `branch-a`: `20`, `140`
- `branch-a-db`: `40`, `120`
- `branch-a-io`: `80`, `80`

## 3. `single_span`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R3["root"]:::critical
```

Expected:
- `root`: `100`, `100`

## 4. `single_child_full_tail`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R4["root"]:::critical --> C4["child"]:::critical
```

Expected:
- `root`: `20`, `100`
- `child`: `80`, `80`

## 5. `single_child_middle_gap`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R5["root"]:::critical --> C5["child"]:::critical
```

Expected:
- `root`: `60`, `100`
- `child`: `40`, `40`

## 6. `two_non_overlapping_siblings`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R6["root"]:::critical --> F6["first"]:::critical
    R6 --> S6["second"]:::critical
```

Expected:
- `root`: `70`, `100`
- `first`: `20`, `20`
- `second`: `10`, `10`

## 7. `overlapping_siblings_latest_only`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R7["root"]:::critical --> F7["first"]:::normal
    R7 --> S7["second"]:::critical
```

Expected:
- `root`: `70`, `120`
- `second`: `50`, `50`

## 8. `three_sibling_staircase`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R8["root"]:::critical --> A8["a"]:::critical
    R8 --> B8["b"]:::critical
    R8 --> C8["c"]:::critical
```

Expected:
- `root`: `100`, `200`
- `a`: `20`, `20`
- `b`: `40`, `40`
- `c`: `40`, `40`

## 9. `nested_chain_with_side_branches`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R9["root"]:::critical --> C9["child"]:::critical
    C9 --> L9["leaf"]:::critical
    R9 --> SR9["side-root"]:::normal
    C9 --> SC9["side-child"]:::normal
```

Expected:
- `root`: `40`, `200`
- `child`: `60`, `160`
- `leaf`: `100`, `100`

## 10. `multi_root_missing_parent`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    RA10["root-a"]:::critical --> CA10["child-a"]:::critical
    RB10["root-b"]:::critical
```

Expected:
- `root-a`: `60`, `100`
- `child-a`: `40`, `40`
- `root-b`: `70`, `70`

## 11. `truncate_child_start`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R11["root"]:::critical --> C11["child"]:::critical
```

Expected:
- `root`: `60`, `90`
- `child`: `30`, `30`

## 12. `truncate_child_end`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R12["root"]:::critical --> C12["child"]:::critical
```

Expected:
- `root`: `70`, `90`
- `child`: `20`, `20`

## 13. `child_covers_parent_after_truncation`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R13["root"]:::critical --> C13["child"]:::critical
```

Expected:
- `root`: `0`, `90`
- `child`: `90`, `90`

## 14. `drop_child_after_parent`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R14["root"]:::critical --> D14["drop"]:::normal
```

Expected:
- `root`: `90`, `90`

## 15. `deep_sibling_hop`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R15["root"]:::critical --> A15["a"]:::critical
    A15 --> B15["b"]:::critical
    R15 --> C15["c"]:::critical
```

Expected:
- `root`: `50`, `200`
- `a`: `60`, `100`
- `b`: `40`, `40`
- `c`: `50`, `50`

## 16. `tie_same_end_later_start_wins`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R16["root"]:::critical --> L16["left"]:::normal
    R16 --> R16B["right"]:::critical
```

Expected:
- `root`: `60`, `100`
- `right`: `40`, `40`

## 17. `tie_same_end_same_start_higher_span_id_wins`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R17["root"]:::critical --> L17["left"]:::normal
    R17 --> R17B["right"]:::critical
```

Expected:
- `root`: `60`, `100`
- `right`: `40`, `40`

## 18. `zero_and_invalid_children`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    R18["root"]:::critical --> C18["child"]:::critical
    R18 --> Z18["zero"]:::normal
    R18 --> I18["invalid"]:::normal
```

Expected:
- `root`: `70`, `100`
- `child`: `30`, `30`

## 19. `transformed_article_tree`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    classDef normal fill:#f5f5f5,stroke:#888,stroke-width:1px;
    N19_1["1"]:::critical --> N19_2["2"]:::critical
    N19_2 --> N19_3["3"]:::critical
    N19_2 --> N19_6["6"]:::normal
    N19_3 --> N19_4["4"]:::normal
    N19_3 --> N19_5["5"]:::critical
    N19_4 --> N19_9["9"]:::normal
    N19_9 --> N19_12["12"]:::normal
    N19_5 --> N19_7["7"]:::critical
    N19_5 --> N19_8["8"]:::normal
    N19_8 --> N19_10["10"]:::normal
    N19_7 --> N19_11["11"]:::critical
    N19_11 --> N19_13["13"]:::critical
    N19_13 --> N19_16["16"]:::critical
    N19_6 --> N19_14["14"]:::normal
    N19_6 --> N19_15["15"]:::normal
```

Expected:
- `1`: `10`, `200`
- `2`: `10`, `190`
- `3`: `10`, `180`
- `5`: `10`, `170`
- `7`: `10`, `160`
- `11`: `10`, `150`
- `13`: `10`, `140`
- `16`: `130`, `130`

## 20. `complex_dual_subtrees`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R20["root"]:::critical --> L20["left"]:::critical
    L20 --> LA20["left-a"]:::critical
    L20 --> LB20["left-b"]:::critical
    R20 --> RT20["right"]:::critical
    RT20 --> RA20["right-a"]:::critical
    RT20 --> RB20["right-b"]:::critical
```

Expected:
- `root`: `60`, `300`
- `left`: `50`, `120`
- `left-a`: `30`, `30`
- `left-b`: `40`, `40`
- `right`: `50`, `120`
- `right-a`: `40`, `40`
- `right-b`: `30`, `30`

## 21. `three_level_sibling_hops`

```mermaid
flowchart TD
    classDef critical fill:#ffe7a3,stroke:#7a5a00,stroke-width:3px;
    R21["root"]:::critical --> A21["a"]:::critical
    A21 --> A121["a1"]:::critical
    A21 --> A221["a2"]:::critical
    R21 --> B21["b"]:::critical
    B21 --> B121["b1"]:::critical
    B21 --> B221["b2"]:::critical
```

Expected:
- `root`: `60`, `250`
- `a`: `30`, `90`
- `a1`: `20`, `20`
- `a2`: `40`, `40`
- `b`: `30`, `100`
- `b1`: `40`, `40`
- `b2`: `30`, `30`

## Source Of Truth

- Executable assertions: [e2e_critical_path_test.go](/Users/israel.blancas/projects/contrib9/processor/coralogixprocessor/e2e_critical_path_test.go)
- Algorithm: [internal/criticalpath/critical_path.go](/Users/israel.blancas/projects/contrib9/processor/coralogixprocessor/internal/criticalpath/critical_path.go)
