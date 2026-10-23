# csv Source

Looks up keys from a CSV file. Supports both CSVs **with a header row** (columns
referenced by name) and **headerless** CSVs (columns referenced by 0-based index),
and returns either a single column as a scalar value (1:1) or the whole row as a map
(1:N). The file is loaded during `Start`; set `reload_interval` to re-read it
periodically without restarting the collector.

## Configuration

| Field                | Description                                                                                                                                                                                                                                                                                                            | Default |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `path`               | Path to the CSV file (required)                                                                                                                                                                                                                                                                                        | -       |
| `has_header`         | Whether the first row is a header of column names. When `false`, columns are referenced by index                                                                                                                                                                                                                       | `true`  |
| `delimiter`          | Field delimiter (single character)                                                                                                                                                                                                                                                                                     | `,`     |
| `key_column`         | Lookup-key column by header name (requires `has_header: true`)                                                                                                                                                                                                                                                         | -       |
| `key_column_index`   | Lookup-key column by 0-based index                                                                                                                                                                                                                                                                                     | -       |
| `value_column`       | Single value column by header name; makes lookups return that column as a scalar (requires `has_header: true`)                                                                                                                                                                                                         | -       |
| `value_column_index` | Single value column by 0-based index                                                                                                                                                                                                                                                                                   | -       |
| `reload_interval`    | If `> 0`, re-read the file on this interval so changes take effect without a collector restart. `0` disables reloading. On a failed reload the previously loaded data is kept and a warning is logged. Update the file atomically (write to a temp file, then rename) so a reload never reads a half-written file. | `0`     |

Exactly one of `key_column` / `key_column_index` is required. At most one of
`value_column` / `value_column_index` may be set; when neither is set, lookups return
the whole row as a `map[string]any` (keyed by column name when `has_header: true`,
otherwise by stringified column index). Column names are resolved against the header on
every (re)load, so reloads tolerate column reordering. Rows shorter than the selected
column are skipped.

## Examples

### Headerless CSV, scalar value (1:1)

CSV file (`stores.csv`):

```csv
1010,closed_store
1012,closed_store
```

Processor config:

```yaml
processors:
  lookup:
    source:
      type: csv
      path: /etc/otel/stores.csv
      has_header: false
      key_column_index: 0
      value_column_index: 1
      reload_interval: 5m
    lookups:
      - key: resource.attributes["store_id"]
        attributes:
          - destination: store_state
            default: "open_store"
```

### CSV with header, scalar value (1:1)

CSV file:

```csv
store_id,store_state
1010,closed_store
```

Processor config:

```yaml
processors:
  lookup:
    source:
      type: csv
      path: /etc/otel/stores.csv
      key_column: store_id
      value_column: store_state
      reload_interval: 5m
    lookups:
      - key: resource.attributes["store_id"]
        attributes:
          - destination: store_state
            default: "open_store"
```

### CSV with header, whole row as a map (1:N)

CSV file:

```csv
store_id,store_state,region
1010,closed_store,NL
```

Processor config:

```yaml
processors:
  lookup:
    source:
      type: csv
      path: /etc/otel/stores.csv
      key_column: store_id
    lookups:
      - key: resource.attributes["store_id"]
        attributes:
          - source: store_state
            destination: store_state
          - source: region
            destination: store.region
```

## Reload metrics

When `reload_interval` is set, each periodic reload increments one of two counters
(shared with the `yaml` source): `lookup_source_reloads` (success) and
`lookup_source_reload_failures`. Alert on the failure counter to detect a stale lookup
table.
