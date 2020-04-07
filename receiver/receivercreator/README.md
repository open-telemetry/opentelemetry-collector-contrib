# receiver_creator

This receiver can instantiate other receivers at runtime.

TODO: More after it can react to observer events.

## Example
```yaml
receivers:
  receiver_creator/1:
    examplereceiver/1:
      rule: <TODO>
      config:
        endpoint: localhost:12345

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
