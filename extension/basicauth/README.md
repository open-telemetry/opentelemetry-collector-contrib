# Basic Authenticator

This extension implements `configauth.ServerAuthenticator` to authenticate clients using HTTP Basic Authentication. The authenticator type has to be set to `basicauth`.

## Configuration

```yaml
extensions:
  basicauth:
    htpasswd: 
      file: .htpasswd
      inline: |
        ${BASIC_AUTH_USERNAME}:${BASIC_AUTH_PASSWORD}

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

### htpasswd 

- `file`: The path to the htpasswd file.
- `inline`: The htpasswd file inline content. 

If both `file` and `inline` are configured, `inline` credentials have a higher priority.