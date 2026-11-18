# Canonical JSON Serialization

This internal package provides RFC 8785 (JCS) canonical JSON serialization for OpenTelemetry log records and spans. It is used by the signing and verify processors to produce deterministic JSON representations for cryptographic operations.

## Purpose

Canonical JSON serialization is required for cryptographic signing and verification of log records. Unlike standard JSON serialization (including OTLP/JSON), RFC 8785 ensures:

1. **Deterministic output**: Same input always produces identical byte-level output
2. **Lexicographic key ordering**: Map keys are sorted alphabetically
3. **Normalized numbers**: Consistent number representation
4. **No whitespace**: Minimal, predictable formatting

## Features

- ✅ RFC 8785 (JCS) compliant canonical JSON
- ✅ Configurable field exclusion (e.g., `audit.integrity.*` attributes)
- ✅ Configurable depth and size limits
- ✅ UTF-8 validation (prevents canonicalization issues)
- ✅ Type-preserving value encoding (e.g., `{"intValue": "42"}`)
- ✅ Comprehensive serialization and validation tests

## Usage

### Basic Serialization

```go
import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"

opts := canonicaljson.SerializeLogRecordOptions{
    ExcludePrefixes: []string{"audit.integrity."},
}

canonical, err := canonicaljson.SerializeLogRecord(logRecord, opts)
if err != nil {
    // Handle error
}
// canonical is deterministic, RFC 8785 compliant JSON
```

### Span Serialization

```go
canonical, err := canonicaljson.SerializeSpan(span, canonicaljson.SerializeSpanOptions{
    ExcludePrefixes: []string{"audit.integrity."},
})
```

`SerializeLogRecordOptions` and `SerializeSpanOptions` are aliases of the shared
`SerializeOptions` type, with the same depth and size defaults. For spans,
`ExcludePrefixes` applies to attributes on the span, its events, and its links.

The hard-coded span schema includes non-empty `trace_id`, `span_id`,
`parent_span_id`, `name`, and filtered `attributes`; non-zero `start_time` and
`end_time`; and always includes numeric `kind` and `status.code`.
`status.message` is included only when non-empty. Non-empty event and link
lists retain their original order, including entries with no included fields.
Events include non-empty names, non-zero timestamps, and filtered attributes;
links include non-empty trace/span IDs and filtered attributes.
Trace state, flags, and all dropped counts are excluded.

### Timestamp Compatibility

All log record and span timestamps are unsigned decimal nanoseconds strings,
preserving the full `uint64` range without converting through `time.Time`.
This applies to log `timestamp` and `observed_timestamp`, span `start_time`
and `end_time`, and span event `timestamp`.

### Custom Limits

```go
opts := canonicaljson.SerializeLogRecordOptions{
    ExcludePrefixes: []string{"audit.integrity.", "audit.verification."},
    MaxDepth:        64,  // Lower depth limit
    MaxInputBytes:   1024 * 1024, // 1 MiB size limit
}
```

### Defaults

```go
// Uses default values if not specified:
// - MaxDepth: 128
// - MaxInputBytes: 2 MiB (1 << 21)
opts := canonicaljson.SerializeLogRecordOptions{
    ExcludePrefixes: []string{"audit.integrity."},
}
```

## Excluded Fields

The following log record fields are **intentionally excluded** from serialization:

### Always Excluded (by design)

- **`SeverityNumber`** - Audit metadata, not part of signed content
- **`SeverityText`** - Audit metadata, not part of signed content
- **`Flags`** - W3C trace-context sampling bit (transport-level metadata)
- **`DroppedAttributesCount`** - Pipeline metadata, not original content

### Conditionally Excluded (via configuration)

- **Attributes matching `ExcludePrefixes`** - e.g., `audit.integrity.*`, `audit.verification.*`

### Design Rationale

Fields are **hard-coded** rather than using reflection for several reasons:

1. **Cryptographic Stability**: Automatically including new OTLP fields would break signature verification for existing records
2. **Explicit Control**: Each field's inclusion is a conscious security decision
3. **Performance**: No reflection overhead during serialization
4. **Audit Trail**: Clear documentation of what is/isn't signed

When new OTLP LogRecord fields are added, they require **explicit review** to determine if they should be part of the signed payload.

## Included Fields

The following log record fields **are serialized** (if non-zero/non-empty):

| Field                 | JSON Key             | Format                     |
|-----------------------|----------------------|----------------------------|
| `EventName()`         | `event_name`         | String                     |
| `Body()`              | `body`               | Type-wrapped value         |
| `Timestamp()`         | `timestamp`          | Unix nanoseconds (string)  |
| `ObservedTimestamp()` | `observed_timestamp` | Unix nanoseconds (string)  |
| `TraceID()`           | `trace_id`           | Hex string                 |
| `SpanID()`            | `span_id`            | Hex string                 |
| `Attributes()`        | `attributes`         | Map of type-wrapped values |

## Value Encoding

Values are wrapped with type information to preserve exact types during verification:

| pcommon.ValueType | JSON Encoding                     |
|-------------------|-----------------------------------|
| `ValueTypeStr`    | `{"stringValue": "text"}`         |
| `ValueTypeInt`    | `{"intValue": "42"}`              |
| `ValueTypeDouble` | `{"doubleValue": 3.14}`           |
| `ValueTypeBool`   | `{"boolValue": true}`             |
| `ValueTypeBytes`  | `{"bytesValue": "base64..."}`     |
| `ValueTypeSlice`  | `[...]` (array of wrapped values) |
| `ValueTypeMap`    | `{...}` (map with wrapped values) |
| `ValueTypeEmpty`  | `null`                            |

## Comparison with OTLP/JSON

| Feature            | Canonical JSON (RFC 8785) | OTLP/JSON           |
|--------------------|---------------------------|---------------------|
| **Purpose**        | Cryptographic signing     | Telemetry transport |
| **Determinism**    | ✅ Guaranteed             | ❌ Not guaranteed   |
| **Key Ordering**   | ✅ Lexicographic          | ❌ Unspecified      |
| **Value Encoding** | Type-wrapped              | Direct              |
| **Use Case**       | Audit integrity           | Data exchange       |

Example:

```json
// Canonical JSON (this package)
{
  "attributes": {
    "http.method": {"stringValue": "GET"},
    "http.status_code": {"intValue": "200"}
  },
  "body": {"stringValue": "Request processed"},
  "timestamp": "1234567890123456789"
}

// OTLP/JSON (standard format)
{
  "body": {"stringValue": "Request processed"},
  "timeUnixNano": "1234567890123456789",
  "attributes": [
    {"key": "http.method", "value": {"stringValue": "GET"}},
    {"key": "http.status_code", "value": {"intValue": 200}}
  ]
}
```

## Error Handling

The package returns errors for:

- **Depth limit exceeded**: Nested structures deeper than `MaxDepth`
- **Size limit exceeded**: Serialized JSON larger than `MaxInputBytes`
- **Invalid UTF-8**: Any string with invalid UTF-8 encoding
- **JCS transformation failure**: Internal JCS library error

## Testing

Run tests:

```bash
cd internal/canonicaljson
go test -race -count=1 -v ./...
```

Test coverage:

- Log record and span serialization, exclusions, and limits (`serialize_test.go`)
- Value conversion (`convert_test.go`)
- UTF-8 validation (`validate_test.go`)
- Boundary and regression cases (`boundaries_test.go`, `regression_test.go`)

## Dependencies

- `github.com/gowebpki/jcs` - RFC 8785 JCS transformation
- `go.opentelemetry.io/collector/pdata` - OpenTelemetry data types

## References

- [RFC 8785 - JSON Canonicalization Scheme (JCS)](https://www.rfc-editor.org/rfc/rfc8785.html)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [OpenTelemetry Audit Logging](https://github.com/open-telemetry/oteps/blob/main/text/logs/)

## Internal Use Only

This package is in `internal/` and is **not part of the public API**. It is designed for use by:

- `processor/signingprocessor` - Signs log records with cryptographic signatures
- `processor/verifyprocessor` - Verifies cryptographic signatures on log records

External projects should **not** depend on this package. If you need canonical JSON serialization, please file an issue to discuss moving it to `pkg/`.

## License

Copyright The OpenTelemetry Authors
SPDX-License-Identifier: Apache-2.0
