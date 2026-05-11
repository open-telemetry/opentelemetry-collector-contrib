# AWS Secrets Manager Auth Extension

| Status        |           |
| ------------- |-----------|
| Stability     | [development] |
| Distributions | [contrib] |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

This extension implements both `extensionauth.Server` and `extensionauth.Client` to authenticate receivers and exporters using HTTP Basic Auth, with credentials sourced from [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) and automatically rotated at a configurable interval.

The extension uses the default AWS credential chain (environment variables, `~/.aws/credentials`, EC2/ECS/EKS IAM roles). No explicit AWS credentials are required in the collector config.

When a new secret version is detected, credentials are swapped atomically with no restart required. If a fetch fails, the extension logs a warning and continues using the last known credentials.

When used as a server authenticator, a successful authentication exposes the following attributes on `client.Info.Auth`:

- `username`: The authenticated username.
- `raw`: Raw base64-encoded credentials.

## Modes

Exactly one of `htpasswd` or `client_auth` must be set.

### Server authentication (`htpasswd`)

The secret value is treated as [htpasswd](https://httpd.apache.org/docs/current/programs/htpasswd.html) file content. All hash formats supported by the [`go-htpasswd`](https://github.com/tg123/go-htpasswd) library are accepted (bcrypt, SHA, MD5, APR1, etc.).

If `value_key` is set, the secret must be a JSON object and the value at that key is used as the htpasswd content. If `value_key` is empty, the entire secret string is used as-is.

### Client authentication (`client_auth`)

The secret must be a JSON object. `username_key` and `password_key` identify which fields to use (both default to `"username"` and `"password"`).

## Configuration

| Field | Required | Default | Description |
|---|---|---|---|
| `secret_arn` | yes | | ARN or name of the secret in AWS Secrets Manager |
| `refresh_interval` | yes | `30s` | How often to poll for a new secret version |
| `htpasswd.value_key` | no | `""` | JSON key holding htpasswd content; if empty the raw secret string is used |
| `client_auth.username_key` | no | `"username"` | JSON key for the username |
| `client_auth.password_key` | no | `"password"` | JSON key for the password |

## Examples

### Client authenticator — outbound requests

Secret value in AWS Secrets Manager:
```json
{"username": "alice", "password": "s3cr3t"}
```

Collector config:
```yaml
extensions:
  awssecretsmanagerauth/client:
    secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-exporter-creds"
    refresh_interval: 30s
    client_auth: {}   # uses "username" and "password" keys by default

exporters:
  otlp:
    endpoint: https://my-collector.example.com:4317
    auth:
      authenticator: awssecretsmanagerauth/client

service:
  extensions: [awssecretsmanagerauth/client]
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [otlp]
```

### Client authenticator — custom JSON key names

Secret value:
```json
{"user": "alice", "pass": "s3cr3t"}
```

```yaml
extensions:
  awssecretsmanagerauth/client:
    secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-creds"
    refresh_interval: 60s
    client_auth:
      username_key: user
      password_key: pass
```

### Server authenticator — inbound requests (raw htpasswd)

Secret value (raw htpasswd content):
```
alice:$2y$05$abcdefghijklmnopqrstuuVGnzYyZ1mBkYHpURsB7L9J2jl5Rp2Cy
bob:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=
```

```yaml
extensions:
  awssecretsmanagerauth/server:
    secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-htpasswd"
    refresh_interval: 30s
    htpasswd: {}   # uses the entire secret string as htpasswd content

receivers:
  otlp:
    protocols:
      http:
        auth:
          authenticator: awssecretsmanagerauth/server

service:
  extensions: [awssecretsmanagerauth/server]
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [debug]
```

### Server authenticator — htpasswd stored as a JSON field

Secret value:
```json
{"htpasswd": "alice:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g="}
```

```yaml
extensions:
  awssecretsmanagerauth/server:
    secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-htpasswd-json"
    refresh_interval: 30s
    htpasswd:
      value_key: htpasswd
```

## Rotation

The extension polls AWS Secrets Manager every `refresh_interval`. AWS rotates secrets by publishing a new secret version; the extension detects the new `VersionId` and swaps credentials atomically without restarting the collector.

To rotate manually:
```bash
aws secretsmanager put-secret-value \
  --secret-id "my-exporter-creds" \
  --secret-string '{"username":"alice","password":"newpassword"}'
```

The collector will pick up the new credentials within one `refresh_interval`.
