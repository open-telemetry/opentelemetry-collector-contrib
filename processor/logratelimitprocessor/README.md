# Log Rate-Limiter Processor

## Details
The logratelimit processor is useful in the situation where you are collecting logs from multiple services on every otel-collector pod and there 
are one or many services which are logging too many logs such that it increases noise in the entire pipeline and storage (otel backend). So to control 
the log noise/surge from any service/namespace/pod etc. you can use the rate-limit processor, you can give the fields on which you want and add config for 
allowed rate and interval.<br>
The processor caches the count of logs in the given interval for each combination of given rate_limit_fields and once logs count starts to exceed the count 
the processor will start dropping the logs till the interval finish in the best effort way. There are no mutex/locks involved, only one atomic counter is used 
to keep rate-limiter lightweight / easy on resources.

## Configuration
| Field             | Type     | Default | Description                                                                                                                                                                            |
|-------------------|----------|-------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| conditions        | []string | `[]`  | A slice of [OTTL] expressions used to evaluate which logs records to be rate-limited. See [OTTL Boolean Expressions] for more details. |
| allowed_rate      | int      | 30000 | Allowed rate of logs per logs combination of rate_limit_fields in configured `interval`                                                                                                |
| interval          | duration | `60s` | The interval in which rate-limit is applied after `allowed_rate`.                                                                                                                      |
| rate_limit_fields | []string | `[]`  | rate-limit is applied for each cobination of values of these fields                                                                                                                    |

[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.109.0/pkg/ottl#readme
[OTTL Boolean Expressions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/LANGUAGE.md#boolean-expressions

### Example Config
The following config is an example configuration for the logratelimit processor. It is configured with an allowed_rate of 30000 in an interval of `60 seconds` for each combination of mentioned rate_limit_fields array.
```yaml
receivers:
    filelog:
        include: [./example/*.log]

processors:
    logratelimit:
        conditions:
          - 'attributes["log_level"] == "error"'
          - 'resource.attributes["k8s.namespace.name"] == "my-k8s-ns-name"'
        allowed_rate: 30000
        interval: 60s
        rate_limit_fields: 
          - attributes.service\.name

exporters:
    kafka:

service:
    pipelines:
        logs:
            receivers: [filelog]
            processors: [logratelimit]
            exporters: [kafka]
```
