# Alert Metrics Extractor Processor

The Alert Metrics Extractor Processor is a filtering processor that dynamically extracts raw metric names from PromQL alert expressions stored in Kubernetes ConfigMaps and filters incoming metrics to only pass those that are referenced in alert definitions.

## Description

This processor reads alert definitions from a Kubernetes ConfigMap, parses PromQL expressions to extract raw metric names, maintains a thread-safe global map of distinct metrics, and filters incoming metrics to only allow those present in the map. It watches for ConfigMap changes and updates the filtering rules in real-time.

**Key Feature**: Automatically converts OpenTelemetry metric names (with dots) to Prometheus-compatible format (with underscores) to ensure proper matching between incoming metrics and PromQL expressions.

## Use Cases

- **Metric Cost Optimization**: Reduce data ingestion costs by only forwarding metrics that are actively used in alerts
- **Performance Improvement**: Decrease processing overhead by filtering out unused metrics early in the pipeline
- **Dynamic Filtering**: Automatically adapt filtering rules when alert definitions change
- **Multi-tenant Environments**: Filter metrics based on tenant-specific alert configurations

## Configuration

The processor can be configured with minimal or detailed configuration. All parameters are optional with sensible defaults.

### Minimal Configuration

```yaml
processors:
  opsrampmetricsfilter:
    # Uses all default values
```

### Detailed Configuration

```yaml
processors:
  opsrampmetricsfilter:
    # Optional: Name of the ConfigMap containing alert definitions
    alert_definitions_configmap_name: "opsramp-alert-user-config"
    
    # Optional: Key in the ConfigMap containing the alert definitions YAML
    alert_definitions_key: "alert-definitions.yaml"
    
    # Note: namespace is automatically read from NAMESPACE environment variable
```

### Configuration Parameters

| Parameter | Type | Default | Required | Description |
|-----------|------|---------|----------|-------------|
| `alert_definitions_configmap_name` | string | `opsramp-alert-user-config` | No | Name of the ConfigMap containing alert definitions |
| `alert_definitions_key` | string | `alert-definitions.yaml` | No | Key in the ConfigMap containing alert definitions YAML |
| `namespace` | string | From `NAMESPACE` env var, fallback to `opsramp-agent` | No | Kubernetes namespace where the ConfigMap is located (auto-populated) |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Kubernetes namespace where ConfigMaps are located | `opsramp-agent` |

The processor automatically reads the namespace from the `NAMESPACE` environment variable, which is typically set by Kubernetes when running as a pod. If the environment variable is not set, it falls back to the default namespace `opsramp-agent`.

## How It Works

### Processing Flow

1. **Initialization**
   - Creates Kubernetes client to access ConfigMaps
   - Reads initial alert definitions from the specified ConfigMap
   - Starts background goroutine to watch for ConfigMap changes

2. **Metric Extraction**
   - Parses YAML alert definitions from ConfigMap
   - For each alert rule, extracts the PromQL expression (`expr` field)
   - Uses Prometheus PromQL parser to walk the AST and identify metric names
   - Handles both `VectorSelector` (instant metrics) and `MatrixSelector` (range metrics)

3. **Global Map Building**
   - Creates a distinct set of all extracted metric names
   - Thread-safely updates the global filtering map
   - Logs the total number of distinct metrics found

4. **Metric Filtering**
   - For each incoming metric batch in the pipeline:
     - **Early Return**: If no metrics are incoming, returns immediately without processing
     - **Drop All**: If no alert definitions are configured, drops all metrics (sends empty batch to exporter)
     - **Prometheus Compatibility**: Converts metric names from OpenTelemetry format (dots) to Prometheus format (underscores)
       - Example: `k8s.pod.cpu.usage` → `k8s_pod_cpu_usage`
     - Checks if the converted metric name exists in the global map
     - **Allows through**: Metrics present in alert definitions
     - **Filters out**: Metrics not referenced in any alert
   - Passes filtered metrics to the next consumer/exporter

5. **Real-time Updates**
   - Continuously watches the ConfigMap for changes
   - When ConfigMap is modified, re-extracts metrics and updates the global map
   - All subsequent filtering uses the updated rules

## Prometheus Compatibility

⚠️ **Important**: This processor automatically converts OpenTelemetry metric names to Prometheus-compatible format by replacing dots (`.`) with underscores (`_`). This ensures that metrics extracted from PromQL expressions (which use underscore notation) correctly match the incoming OpenTelemetry metrics (which may use dot notation).

### Metric Name Conversion Examples

| OpenTelemetry Format | Prometheus Format | Match Result |
|---------------------|------------------|--------------|
| `k8s.pod.cpu.usage` | `k8s_pod_cpu_usage` | ✅ Matches |
| `system.disk.io.read.bytes` | `system_disk_io_read_bytes` | ✅ Matches |
| `http_requests_total` | `http_requests_total` | ✅ Matches (no change needed) |

Without this conversion, OpenTelemetry metrics with dots would not match PromQL expressions that reference them with underscores, causing all metrics to be filtered out.

### Alert Definitions Format

The processor expects alert definitions in the following YAML structure:

```yaml
alertDefinitions:
  - resourceType: "k8s_cluster"
    rules:
      - name: "api_server_availability"
        expr: "(sum(increase(apiserver_request_total{verb!=\"WATCH\"}[5m])) / sum(increase(apiserver_request_total{verb!=\"WATCH\"}[5m]))) * 100"
        # ... other alert fields
      
  - resourceType: "k8s_node"
    rules:
      - name: "node_memory_usage"
        expr: "(k8s_node_memory_working_set / k8s_node_memory_available) * 100"
        # ... other alert fields
```

### PromQL Metric Extraction

The processor can extract metrics from various PromQL constructs:

- **Simple metrics**: `up`, `cpu_usage_percent`
- **Metrics with labels**: `apiserver_request_total{verb="GET"}`
- **Function calls**: `rate(http_requests_total[5m])`, `increase(errors_total[1h])`
- **Range queries**: `node_cpu_seconds_total[5m]`
- **Complex expressions**: `(1 - (node_memory_available / node_memory_total)) * 100`
- **Aggregations**: `sum(rate(http_requests_total[5m]))`

#### Example Extraction

From this PromQL expression:
```promql
(sum(increase(apiserver_request_total{verb!="WATCH"}[5m])) / sum(increase(apiserver_request_total{verb!="WATCH"}[5m]))) * 100
```

The processor extracts: `["apiserver_request_total"]`

From this expression:
```promql
rate(http_requests_total[5m]) + rate(http_errors_total[5m])
```

The processor extracts: `["http_requests_total", "http_errors_total"]`

## Usage Examples

### Basic Pipeline Configuration

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'kubernetes-metrics'
          kubernetes_sd_configs:
            - role: node

processors:
  opsrampmetricsfilter:
    alert_definitions_configmap_name: "opsramp-alert-user-config"
    alert_definitions_key: "alert-definitions.yaml"
    namespace: "opsramp-agent"
  
  batch:
    timeout: 1s

exporters:
  prometheusremotewrite:
    endpoint: "https://prometheus.example.com/api/v1/write"

service:
  pipelines:
    metrics/filtered:
      receivers: [prometheus]
      processors: [opsrampmetricsfilter, batch]
      exporters: [prometheusremotewrite]
```

### ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: opsramp-alert-user-config
  namespace: opsramp-agent
data:
  alert-definitions.yaml: |
    alertDefinitions:
      - resourceType: k8s_cluster
        rules:
          - name: k8s_apiserver_requests_error_rate
            expr: (sum(increase(apiserver_request_total{verb!="WATCH",code=~"2.."}[5m])) / sum(increase(apiserver_request_total{verb!="WATCH"}[5m]))) * 100
          - name: k8s_cluster_nodes_health
            expr: (sum(k8s_node_condition_ready) / count(k8s_node_condition_ready)) * 100
      - resourceType: k8s_pod
        rules:
          - name: k8s_pod_cpu_usage
            expr: k8s_pod_cpu_limit_utilization_ratio * 100
```

## Thread Safety

The processor ensures thread-safe operation through:

- **Read-Write Mutex**: Uses `sync.RWMutex` for the global metrics map
- **Concurrent Reads**: Multiple goroutines can read the map simultaneously during metric filtering
- **Exclusive Writes**: ConfigMap updates acquire exclusive write access to update the map
- **Atomic Operations**: Map updates are atomic to prevent race conditions

## Filtering Behavior

### When No Alert Definitions Are Configured

The processor implements a **"drop-all"** approach when no alert definitions are available:

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| ConfigMap not found | **Drops all metrics** | No alert definitions = no metrics needed |
| ConfigMap key missing | **Drops all metrics** | No alert definitions = no metrics needed |
| Empty alert definitions | **Drops all metrics** | No expressions = no metrics to extract |
| Invalid PromQL expressions | **Drops all metrics** | Cannot extract metrics from invalid expressions |
| ConfigMap deleted | **Drops all metrics** | Alert definitions no longer available |

This behavior ensures that:
- **Cost optimization**: No unnecessary metrics are sent to expensive storage/monitoring systems
- **Performance**: Minimal resource usage when no filtering rules are defined
- **Fail-safe operation**: Prevents accidental metric flooding when configuration is missing

### Early Return Optimizations

The processor includes several optimizations to skip unnecessary processing:

- **No incoming metrics**: Returns immediately if the incoming metric batch is empty
- **No configured filters**: Drops all metrics without iterating through them
- **Empty metric names**: Skips metrics with empty names during filtering

## Error Handling

| Error Type | Behavior | Log Level |
|------------|----------|-----------|
| ConfigMap not found | Processor continues with empty map (drops all metrics) | Error |
| Invalid YAML | Processor continues with previous map | Error |
| PromQL parse error | Skips that expression, continues with others | Warning |
| Kubernetes API error | Retries with exponential backoff | Error |
| ConfigMap deleted | Clears metrics map (drops all metrics) | Warning |

## Performance Considerations

- **Memory Usage**: Only stores metric names (strings), not metric data
- **CPU Impact**: PromQL parsing done only when ConfigMap changes, not per metric
- **Filtering Performance**: O(1) map lookup per metric name
- **Network**: Minimal - only watches ConfigMap changes via Kubernetes API
- **Scalability**: Handles thousands of metrics efficiently

## Troubleshooting

### Common Issues

1. **No metrics passing through**
   - Check if ConfigMap exists and is accessible
   - Verify alert definitions contain valid PromQL expressions
   - Check processor logs for parsing errors
   - **Most Common**: Ensure your PromQL expressions use underscore notation (e.g., `k8s_pod_cpu_usage` not `k8s.pod.cpu.usage`)
   - **New behavior**: If no alert definitions are configured, ALL metrics are dropped by design

2. **All metrics being dropped unexpectedly**
   - **ConfigMap missing**: Verify the ConfigMap exists in the specified namespace
   - **Key missing**: Check that the `alert_definitions_key` matches the actual key in the ConfigMap
   - **Empty definitions**: Ensure the ConfigMap contains valid alert definitions with PromQL expressions
   - **Invalid expressions**: Check processor logs for PromQL parsing errors
   - This is the expected behavior when no valid alert definitions are found

3. **OpenTelemetry metrics not matching PromQL expressions**
   - This is automatically handled by the processor's dot-to-underscore conversion
   - If still having issues, verify the converted metric names in debug logs
   - Ensure your alert expressions use standard Prometheus naming conventions

4. **ConfigMap changes not detected**
   - Ensure proper RBAC permissions for ConfigMap access
   - Check if ConfigMap is in the correct namespace
   - Verify Kubernetes watch connection

5. **Some metrics still filtered**
   - Check if metric names match exactly (case-sensitive)
   - Verify PromQL expressions reference the expected metrics
   - Enable debug logging to see extracted metric names

6. **Metrics not reaching Prometheus remote write**
   - This was a common issue before the dot-to-underscore conversion was implemented
   - Ensure you're using the latest version of the processor
   - Check logs for "Metrics filtering completed" to see if metrics are being filtered correctly

### Debug Logging

Enable debug logging to see detailed information:

```yaml
service:
  telemetry:
    logs:
      level: debug
```

This will show:
- Extracted metric names from each alert expression
- ConfigMap watch events
- Filtering decisions for each metric
- Dot-to-underscore conversion details

### Typical Log Output

```
2024-01-15T10:30:00Z info Loading alert definitions from ConfigMap configmap=opsramp-alert-user-config namespace=opsramp-agent
2024-01-15T10:30:00Z info Successfully loaded alert definitions alert_definitions=2 unique_metrics=15
2024-01-15T10:30:05Z info Metrics filtering completed incoming_metrics=1250 filtered_metrics=15 configured_filter_count=15
```

**When no alert definitions are configured:**
```
2024-01-15T10:30:00Z error Failed to get ConfigMap configmap=opsramp-alert-user-config namespace=opsramp-agent error="configmaps \"opsramp-alert-user-config\" not found"
2024-01-15T10:30:05Z info No metrics filter configured, dropping all metrics dropped_metrics=1250
```

**When no incoming metrics:**
```
2024-01-15T10:30:05Z debug No incoming metrics to process
```

## RBAC Requirements

The processor requires the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: otelcol-opsrampmetricsfilter
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
```