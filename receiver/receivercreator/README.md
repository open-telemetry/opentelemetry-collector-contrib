# receiver_creator

This receiver can instantiate other receivers at runtime based on whether observed endpoints match a configured rule.

## TODO
* Observer endpoint details
* Rule matching

## Example
```yaml
extensions:
  # Configures the Kubernetes observer to watch for pod start and stop events.
  k8s_observer:

receivers:
  receiver_creator/1:
    # Name of the extensions to watch for endpoints to start and stop.
    watch_observers: [k8s_observer]
    receivers:
        redis/1:
          # If this rule matches an instance of this receiver will be started.
          rule: port == 6379
          config:
            # Static receiver-specific config.
            password: secret

processors:
  exampleprocessor:

exporters:
  exampleexporter:

service:
  pipelines:
    metrics:
      receivers: [receiver_creator/1]
      processors: [exampleprocessor]
      exporters: [exampleexporter]
```
