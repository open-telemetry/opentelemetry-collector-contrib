# Basic Authenticator

This extension implements `configauth.ServerAuthenticator` to authenticate clients using HTTP Basic Authentication. The authenticator type has to be set to `basicauth`.

## Configuration

The following settings are available:

- `htpasswd` - Path to the htpasswd file used to store the credentials (defaults to `.htpasswd`).

### Example

```yaml
extensions:
  basicauth:
    htpasswd: .htpasswd

receivers:
  otlp:
    protocols:
      http:
        auth:
          authenticator: basicauth

processors:

exporters:
  logging:
    logLevel: debug

service:
  extensions: [basicauth]
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [logging]
```

