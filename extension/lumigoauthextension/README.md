# Lumigo Authenticator

| Status                   |                      |
| ------------------------ |----------------------|
| Stability                | [stable]              |
| Supported pipeline types | trace                |
| Distributions            | [lumigo]             |

This extension implements the `configauth.ServerAuthenticator` interface to require clients to send a Lumigo token with a header with the following structure:

```
Authentication: LumigoToken <token>
```

If the `Authentication` header is not provided, ot its structure does not match the expected pattern, a `401` is returned.

The extension does not yet perform validation of the token itself.

The token is available in the `auth` context with the `auth.lumigo-token` key.
The raw value of the `Authentication` header is available as `auth.raw`.

## Configuration

The following configuration includes the setting of the token as an additional resource attribute called `lumigoToken` via the [`resourceprocessor`](../../processor/resourceprocessor/) processor:

```yaml
extensions:
  lumigoauth: {}

receivers:
  otlp:
    protocols:
      http:
        auth:
          authenticator: lumigoauth

processors:
  resource/lumigo_token_from_auth:
    attributes:
    - key: lumigoToken
      action: upsert
      from_context: auth.lumigo-token

exporters:
  otlp:

service:
  extensions: [lumigoauth]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [resource/lumigo_token_from_auth]
      exporters: [otlp]
```

[stable]:https://github.com/open-telemetry/opentelemetry-collector#stableq
[lumigo]:https://github.com/lumigo-io/opentelemetry-collector-contrib