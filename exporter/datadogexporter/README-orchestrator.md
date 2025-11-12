# DataDog Orchestrator Exporter

The DataDog Orchestrator Exporter is an extension to the DataDog exporter that allows you to export Kubernetes resource manifests to DataDog's Orchestrator endpoint. This enables you to visualize and monitor your Kubernetes cluster resources in DataDog's Orchestrator view.

## Features

- Export Kubernetes resource manifests (Pods, Deployments, Services, etc.) to DataDog
- Support for both pull and watch modes for Kubernetes objects
- Configurable compression and batching
- Leader election support for high availability deployments
- Cluster checks and events collection
- Custom cluster naming and tagging

## Configuration

### Basic Configuration

```yaml
exporters:
  datadog:
    api:
      key: ${env:DD_API_KEY}
      site: ${env:DD_SITE}
    orchestrator:
      cluster_name: "my-cluster"
      use_compression: true
      compression_level: 6
      batch_wait: 5
      cluster_checks_enabled: true
      collect_events: true
      leader_election: false
```

### Complete Example

```yaml
receivers:
  k8sobjects:
    auth_type: serviceAccount
    objects:
      - name: pods
        mode: pull
        interval: 15s
      - name: deployments
        mode: pull
        interval: 15s
      - name: services
        mode: pull
        interval: 15s
      - name: nodes
        mode: pull
        interval: 15s
      - name: namespaces
        mode: pull
        interval: 15s
      - name: events
        mode: watch
        group: events.k8s.io
        namespaces: [default]

processors:
  resource:
    attributes:
      - key: deployment.environment
        value: "production"
        action: upsert
      - key: service.version
        value: "v1.0.0"
        action: upsert
  batch:
    send_batch_size: 1024
    timeout: 10s

exporters:
  datadog:
    api:
      key: ${env:DD_API_KEY}
      site: ${env:DD_SITE}
    orchestrator:
      cluster_name: "my-cluster"
      use_compression: true
      compression_level: 6
      batch_wait: 5
      cluster_checks_enabled: true
      collect_events: true
      leader_election: false

service:
  pipelines:
    orchestrator:
      receivers: [k8sobjects]
      processors: [resource, batch]
      exporters: [datadog]
```

## Configuration Options

### Orchestrator Configuration

| Option                   | Type   | Default                              | Description                                 |
| ------------------------ | ------ | ------------------------------------ | ------------------------------------------- |
| `endpoint`               | string | `https://orchestrator.datadoghq.com` | The DataDog orchestrator endpoint           |
| `use_compression`        | bool   | `true`                               | Enable compression for data transmission    |
| `compression_level`      | int    | `6`                                  | Compression level (0-9)                     |
| `batch_wait`             | int    | `5`                                  | Maximum time to wait before sending a batch |
| `cluster_name`           | string | `""`                                 | Name of the Kubernetes cluster              |
| `cluster_checks_enabled` | bool   | `true`                               | Enable cluster checks                       |
| `collect_events`         | bool   | `true`                               | Enable collection of Kubernetes events      |
| `leader_election`        | bool   | `false`                              | Enable leader election                      |
| `leader_lease_duration`  | int    | `15`                                 | Leader lease duration in seconds            |
| `leader_renew_deadline`  | int    | `10`                                 | Leader renew deadline in seconds            |
| `leader_retry_period`    | int    | `2`                                  | Leader retry period in seconds              |

## Kubernetes RBAC Requirements

The orchestrator exporter requires the following RBAC permissions to collect Kubernetes resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: otel-collector-orchestrator
rules:
  - apiGroups: [""]
    resources:
      - pods
      - services
      - nodes
      - namespaces
      - events
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources:
      - deployments
      - replicasets
      - statefulsets
      - daemonsets
    verbs: ["get", "list", "watch"]
  - apiGroups: ["events.k8s.io"]
    resources:
      - events
    verbs: ["get", "list", "watch"]
```

## DataDog API Key

You need a DataDog API key with the following permissions:

- `orchestrator_read` - Read orchestrator data
- `orchestrator_write` - Write orchestrator data

## Troubleshooting

### Common Issues

1. **Authentication Errors**: Ensure your DataDog API key is valid and has the required permissions.

2. **RBAC Errors**: Verify that the service account has the necessary RBAC permissions to access Kubernetes resources.

3. **Network Issues**: Check that the collector can reach the DataDog orchestrator endpoint.

4. **Data Not Appearing**: Verify that the pipeline is correctly configured and the orchestrator pipeline is enabled.

### Logs

The orchestrator exporter will log errors and warnings related to:

- Failed API requests to DataDog
- Invalid Kubernetes resource data
- Network connectivity issues
- Configuration problems

## Monitoring

You can monitor the orchestrator exporter using:

- DataDog logs for export errors
- Kubernetes events for RBAC issues
- OpenTelemetry metrics for pipeline performance

## Support

For issues and questions:

1. Check the DataDog documentation
2. Review the OpenTelemetry Collector logs
3. Verify your configuration against the examples
4. Contact DataDog support if needed
