# Leader Elector Extension

This extension enables OpenTelemetry components to run in HA mode across a Kubernetes cluster. The component that owns the lease becomes the leader and becomes the active instance.
## How It Works

The extension uses k8s.io/client-go/tools/leaderelection to perform leader election. The component that owns the lease becomes the leader and runs the function defined in onStartedLeading. If the leader loses the lease, it runs the function defined in onStoppedLeading, stops its operation, and waits to acquire the lease again.
## Configuration

```yaml
receivers:
  my_awesome_reciever/foo:
    auth_type: kubeConfig
    lease:
      name: leaderelector/foo
extensions:
  leaderelector/foo:
    auth_type: kubeConfig
    lease_name: foo
    lease_namespace: default

service:
  extensions: [leaderelector/foo]
  pipelines:
    metrics:
      receivers: [my_awesome_receiver/foo]
```

### Leader Election Configuration
| configuration       | description                                                                   | default value   |
|---------------------|-------------------------------------------------------------------------------|-----------------|
| **auth_type**       | Authorization type to be used (serviceAccount, kubeConfig).                   | none (required) |
| **lease_name**      | The name of the lease object.                                                 | none (required) |
| **lease_namespace** | The namespace of the lease object.                                            | none (required) |
| **lease_duration**  | The duration of the lease.                                                    | 15s             |
| **renew_deadline**  | The deadline for renewing the lease. It must be less than the lease duration. | 10s             |
| **retry_period**    | The period for retrying the leader election.                                  | 2s              |
