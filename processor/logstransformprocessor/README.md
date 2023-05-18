# Logs Transform Processor

| Status                   |                      |
|--------------------------|----------------------|
| Stability                | [development]        |
| Supported pipeline types | logs                 |
| Distributions            | [splunk], [observiq] |

NOTE - This processor is experimental, with the intention that its functionality will be reimplemented in the [transform processor](../transformprocessor/README.md) in the future.

The logs transform processor can be used to apply [log operators](../../pkg/stanza/docs/operators) to logs coming from any receiver.
Please refer to [config.go](./config.go) for the config spec.

Examples:

```yaml
processors:
  logstransform:
    operators:
      - type: regex_parser
        regex: '^(?P<time>\d{4}-\d{2}-\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$'
        timestamp:
          parse_from: body.time
          layout: '%Y-%m-%d'
        severity:
          parse_from: body.sev
```

Refer to [config.yaml](./testdata/config.yaml) for detailed
examples on using the processor.

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[splunk]: https://github.com/signalfx/splunk-otel-collector
[observiq]: https://github.com/observIQ/observiq-otel-collector
