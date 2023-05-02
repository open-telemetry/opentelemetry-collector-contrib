# jq processor

Supported pipeline types: logs
Status: in development

This processor allows using jq on json bodies of logs and emitting that jq output as logs 

Typical use-cases:

* JSON sanitization
* Modification/Addition of JSON attributes
* Splitting of JSON arrays into multiple separate logs

Please refer to [config.go](./config.go) for the config spec.

Examples:

```yaml
processors:
  jqprocessor:
    - jq_statement: '.arbitraryJsonArray[]'
```

## Configuration

Refer to [config.yaml](./testdata/config.yaml) for detailed examples on using the processor.

The `jq_statement` property takes a valid jq input, performs it on the body of a log, and sends the result on through the pipeline.


[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib