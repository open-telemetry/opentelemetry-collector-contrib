# Leader Elector Extension

This extension enables executing multiple receivers in a HA mode. The receiver which wins the lease becomes leader and thus becomes active.

## How It Works

It utilizes leader election to determine which instance should be the leader. The receiver that wins the lease becomes the leader and starts executing function defined in `onStartedLeading`. When the leader loses the lease, it executes function defined in `onStoppedLeading`, stops the receiver and waits until it acquires the lease again.

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
| configuration       | description                                                                     | default value   |
|---------------------|---------------------------------------------------------------------------------|-----------------|
| **auth_type**       | Authorization type to be used.                                                  | none (required) |
| **lease_name**      | The name of the lease object.                                                   | none (required) |
| **lease_namespace** | The namespace of the lease object.                                              | none (required) |
| **lease_duration**  | The duration of the lease.                                                      | 15s             |
| **renew_deadline**  | The deadline for renewing the lease. It should be less than the lease duration. | 10s             |
| **retry_period**    | The period for retrying the leader election.                                    | 2s              |
