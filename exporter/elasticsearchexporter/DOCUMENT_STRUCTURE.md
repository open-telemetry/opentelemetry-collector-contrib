# Elasticsearch Exporter - Document Structure Examples

This document shows the actual JSON document structure indexed to Elasticsearch under different mapping modes. These examples are automatically generated from test data and validated in CI to ensure they always reflect the current implementation.

## Overview

The Elasticsearch exporter supports multiple mapping modes, each producing different document structures:

- **`otel`** (default, recommended): OTel-native schema with original attribute names
- **`ecs`**: Maps OpenTelemetry Semantic Conventions to Elastic Common Schema
- **`none`**: Original OTLP field names with `Attributes.` prefix
- **`raw`**: Like `none` but without prefixes
- **`bodymap`**: Log body becomes the document directly (logs only)

## Document Structure by Mapping Mode

### OTel Mapping Mode (Recommended)

The default and recommended "OTel-native" mapping mode stores documents in Elastic's preferred schema.

**Supported Signals**: Logs ✓ | Traces ✓ | Metrics ✓ | Profiles ✓

**Key Features**:
- Original attribute names preserved
- Closely follows OTLP event structure
- Special handling for `data_stream.*` attributes
- Span events stored as separate documents
- Dataset automatically appended with `.otel`

**Example Files**:
- [Logs](testdata/golden/otel_logs.json)
- [Traces](testdata/golden/otel_traces.json)
- [Metrics](testdata/golden/otel_metrics.json)

**Key Fields**:
```
@timestamp                    - Event timestamp
observed_timestamp            - When log was observed (logs only)
body                          - Log message content (logs)
severity_text / severity_number - Log severity (logs)
trace_id / span_id            - Trace context
name                          - Span name (traces)
kind                          - Span kind (traces)
duration                      - Span duration in microseconds (traces)
status.*                      - Span/log status
resource_attributes.*         - Resource-level attributes
scope_attributes.*            - Instrumentation scope attributes
log_attributes.*              - Log-specific attributes (logs)
span_attributes.*             - Span-specific attributes (traces)
metrics.*                     - Metric values (metrics)
events                        - Span events array (traces)
links                         - Span links array (traces)
data_stream.*                 - Data stream routing fields
```

### ECS Mapping Mode

Maps OpenTelemetry Semantic Conventions to Elastic Common Schema for compatibility with existing Elastic dashboards.

**Supported Signals**: Logs ✓ | Traces ✓ | Metrics ✓

**Key Features**:
- Maps OTel SemConv to ECS field names
- Compatible with Elastic dashboards
- Preserves some original attributes
- Compound field mappings (e.g., `host.name` and `host.hostname`)

**Example Files**:
- [Logs](testdata/golden/ecs_logs.json)
- [Traces](testdata/golden/ecs_traces.json)
- [Metrics](testdata/golden/ecs_metrics.json)

**Key ECS Fields**:
```
@timestamp                    - Event timestamp
message                       - Log message (logs)
log.level                     - Severity text (logs)
event.severity                - Severity number (logs)
event.action                  - Event name
event.outcome                 - Success/failure (traces)
trace.id / span.id            - Trace context
span.name                     - Span name (traces)
span.kind                     - Span kind (traces)
span.db.*                     - Database fields (traces)
service.*                     - Service identification
host.*                        - Host information
http.*                        - HTTP-specific fields
error.*                       - Error information
client.ip / source.ip         - Network addresses
kubernetes.*                  - Kubernetes metadata
cloud.*                       - Cloud provider metadata
data_stream.*                 - Data stream routing fields
```

### None Mapping Mode

Preserves original OTLP structure with `Attributes.` prefix for attributes and `Events.` prefix for span events.

**Supported Signals**: Logs ✓ | Traces ✓

**Example Files**:
- [Logs](testdata/golden/none_logs.json)
- [Traces](testdata/golden/none_traces.json)

**Key Fields**:
```
@timestamp                    - Event timestamp
TraceId / SpanId              - Trace context
SeverityText / SeverityNumber - Log severity (logs)
Body                          - Log message (logs)
Name                          - Span name (traces)
Kind                          - Span kind (traces)
Duration                      - Span duration (traces)
TraceStatus                   - Status code
Resource                      - Resource attributes map
Scope                         - Scope attributes map
Attributes.*                  - Signal-specific attributes
Events.*                      - Span events (traces)
```

### Raw Mapping Mode

Similar to `none` mode but without the `Attributes.` and `Events.` prefixes.

**Supported Signals**: Logs ✓ | Traces ✓

**Example Files**:
- [Logs](testdata/golden/raw_logs.json)
- [Traces](testdata/golden/raw_traces.json)

### Bodymap Mapping Mode

The log record body becomes the exact content of the Elasticsearch document without transformation. Intended for use cases where the client has complete control over document structure.

**Supported Signals**: Logs ✓

**Example Files**:
- [Logs](testdata/golden/bodymap_logs.json)

**Requirements**:
- Log body must be of type `Map`
- All fields come directly from the log body
- No automatic field additions

## Generating and Validating Examples

These examples are maintained through automated golden file tests.

### Update Golden Files

After making changes to document encoding logic:

```bash
cd exporter/elasticsearchexporter
make golden-update
```

This will regenerate all example files in `testdata/golden/`.

### Validate Golden Files

To ensure current code matches the documented structure:

```bash
cd exporter/elasticsearchexporter
make golden-validate
```

This runs automatically in CI and will fail if document structure changes without updating golden files.

### Manual Test Execution

```bash
# Update golden files
go test -tags=integration -run TestDocumentStructureGolden -update-golden -v

# Validate golden files
go test -tags=integration -run TestDocumentStructureGolden -v
```

## CI/CD Integration

The golden file validation runs automatically on pull requests that modify the Elasticsearch exporter. If the document structure changes:

1. The CI build will fail
2. Error message will indicate which files differ
3. Run `make golden-update` to regenerate examples
4. Review changes and commit updated golden files

## See Also

- [Main README](README.md) - Configuration and usage documentation
- [ECS Mapping Details](README.md#ecs-mapping) - Detailed attribute conversion tables
- [Golden Test Implementation](golden_test.go) - Test code
- [Sample Data Helpers](testdata_helpers.go) - Sample data generation
