# Headers Setter extension

| Status        |           |
|---------------|-----------|
| Stability     | [alpha]   |
| Distributions | [contrib] |

The `headers_setter` extension implements `ClientAuthenticator` and is used to
set requests headers in `gRPC` / `HTTP` exporters with values provided via
extension configurations or requests metadata (context).

Use cases include but are not limited to enabling multi-tenancy for observability
backends such as [Tempo], [Mimir], [Loki] and others by setting the `X-Scope-OrgID`
header to the value extracted from the context.

## Configuration

The following settings are required:

- `headers`: a list of header configuration objects that specify headers and 
   their value sources. Each configuration object has the following properties:
    - `key`: the header name
    - `value`: the header value is looked up from the `value` property of the
       extension configuration
    - `from_context`: the header value is looked up from the request metadata,
       such as HTTP headers, using the property value as the key (likely a header name)

The `value` and `from_context` properties are mutually exclusive.


#### Configuration Example

```yaml
extensions:
  headers_setter:
    headers:
      - key: X-Scope-OrgID
        from_context: tenant_id
      - key: User-ID
        value: user_id

receivers:
  otlp:
    protocols:
      http:
        include_metadata: true

processors:
  nop:

exporters:
  loki:
    labels:
      resource:
        container_id: ""
        container_name: ""
    endpoint: https://localhost:<port>/loki/api/v1/push
    auth:
      authenticator: headers_setter

service:
  extensions: [ headers_setter ]
  pipelines:
    traces:
      receivers: [ otlp ]
      processors: [ nop ]
      exporters: [ loki ]
```

## Limitations

At the moment, it is not possible to use the `from_context` option to ge the
header value if Collector's pipeline contains the batch processor. See [#4544].


[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[Mimir]: https://grafana.com/oss/mimir/
[Tempo]: https://grafana.com/oss/tempo/
[Loki]: https://grafana.com/oss/loki/
[#4544]: https://github.com/open-telemetry/opentelemetry-collector/issues/4544
