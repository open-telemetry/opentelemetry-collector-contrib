# AWS Secrets Manager Authenticator Extension

| Status        |           |
|---------------|-----------|
| Stability     | [alpha]   |
| Distributions | [contrib] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Aextension%2Fawssecretsmanagerauth%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Aextension%2Fawssecretsmanagerauth) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Aextension%2Fawssecretsmanagerauth%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Aextension%2Fawssecretsmanagerauth) |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

This extension provides Basic Auth authentication with credentials sourced from
AWS Secrets Manager and rotated in place without collector restarts.

It supports both **client-side** (outbound requests) and **server-side** (inbound
request validation) modes, mutually exclusive per instance.

## Configuration

### Client mode

Fetches a JSON secret containing username and password fields:

```yaml
extensions:
  awssecretsmanagerauth:
    client_auth:
      secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-creds"
      region: "us-east-1"
      username_key: "username"
      password_key: "password"
      refresh_interval: 5m

exporters:
  otlphttp:
    endpoint: https://backend.example.com
    auth:
      authenticator: awssecretsmanagerauth
```

### Server mode

Fetches htpasswd content from a secret (raw string or JSON field):

```yaml
extensions:
  awssecretsmanagerauth:
    htpasswd:
      secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-htpasswd"
      region: "us-east-1"
      refresh_interval: 5m

receivers:
  otlp:
    protocols:
      http:
        auth:
          authenticator: awssecretsmanagerauth
```

If the secret is stored as JSON with the htpasswd content in a specific field:

```yaml
extensions:
  awssecretsmanagerauth:
    htpasswd:
      secret_arn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-json"
      region: "us-east-1"
      value_key: "htpasswd_content"
      refresh_interval: 5m
```

### Configuration fields

| Field | Mode | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `secret_arn` | both | yes | | ARN of the AWS Secrets Manager secret |
| `region` | both | yes | | AWS region of the secret |
| `refresh_interval` | both | no | `5m` | How often to re-fetch the secret |
| `username_key` | client | yes | | JSON key for the username |
| `password_key` | client | yes | | JSON key for the password |
| `value_key` | server | no | | JSON key containing htpasswd content (if secret is JSON) |

## AWS Authentication

This extension uses the AWS SDK default credential chain. Ensure the collector
has appropriate IAM permissions:

```json
{
  "Effect": "Allow",
  "Action": "secretsmanager:GetSecretValue",
  "Resource": "arn:aws:secretsmanager:REGION:ACCOUNT:secret:SECRET_NAME"
}
```

## Architecture

### Client Mode (Outbound Authentication)

```
OTel Collector       Auth Extension       AWS Provider        AWS Secrets Manager     Exporter        Backend
(startup)            (client mode)        (refresh loop)      (API)                   (HTTP/gRPC)     (destination)
     │                    │                    │                    │                    │                │
     │  CreateExtension   │                    │                    │                    │                │
     │───────────────────▶│                    │                    │                    │                │
     │                    │  NewProvider        │                    │                    │                │
     │                    │───────────────────▶│                    │                    │                │
     │  Start             │                    │                    │                    │                │
     │───────────────────▶│                    │                    │                    │                │
     │                    │  Start             │                    │                    │                │
     │                    │───────────────────▶│                    │                    │                │
     │                    │                    │  GetSecretValue    │                    │                │
     │                    │                    │───────────────────▶│                    │                │
     │                    │                    │◀───────────────────│                    │                │
     │                    │                    │  (parse JSON:      │                    │                │
     │                    │                    │   username_key,    │                    │                │
     │                    │                    │   password_key)    │                    │                │
     │                    │  FetchFunc(creds)  │                    │                    │                │
     │                    │◀───────────────────│                    │                    │                │
     │                    │  atomic.Store      │                    │                    │                │
     │                    │───────────────────▶│                    │                    │                │
     │                    │                    │  StartTicker(5m)   │                    │                │
     │                    │                    │─────────┐          │                    │                │
     │                    │                    │◀────────┘          │                    │                │
     │                    │                    │                    │                    │                │
     │                    │                    │                    │  SendData          │                │
     │                    │                    │                    │◀───────────────────│                │
     │                    │  RoundTripper()    │                    │                    │                │
     │                    │◀─────────────────────────────────────────────────────────────│                │
     │                    │  Username/Password │                    │                    │                │
     │                    │  (atomic.Load)     │                    │                    │                │
     │                    │─────────────────────────────────────────────────────────────▶│                │
     │                    │                    │                    │                    │  HTTP+BasicAuth│
     │                    │                    │                    │                    │───────────────▶│
     │                    │                    │                    │                    │◀───────────────│
     │                    │                    │                    │                    │                │
     │                    │                    │  ── REFRESH ──     │                    │                │
     │                    │                    │  (ticker fires)    │                    │                │
     │                    │                    │  GetSecretValue    │                    │                │
     │                    │                    │───────────────────▶│                    │                │
     │                    │                    │◀───────────────────│                    │                │
     │                    │  FetchFunc(new)    │                    │                    │                │
     │                    │◀───────────────────│                    │                    │                │
     │                    │  atomic.Store      │                    │                    │                │
     │                    │───────────────────▶│                    │                    │                │
```

### Server Mode (Inbound Authentication)

```
Client/Agent         OTel Receiver        Auth Extension       AWS Provider        AWS Secrets Manager
(sending data)       (HTTP/gRPC)          (server mode)        (refresh loop)      (API)
     │                    │                    │                    │                    │
     │                    │                    │  Start             │                    │
     │                    │                    │───────────────────▶│                    │
     │                    │                    │                    │  GetSecretValue    │
     │                    │                    │                    │───────────────────▶│
     │                    │                    │                    │◀───────────────────│
     │                    │                    │                    │  (parse htpasswd   │
     │                    │                    │                    │   or JSON+value_key)│
     │                    │                    │  FetchFunc(matcher)│                    │
     │                    │                    │◀───────────────────│                    │
     │                    │                    │  atomic.Store      │                    │
     │                    │                    │  (matchFunc)       │                    │
     │                    │                    │                    │  StartTicker(5m)   │
     │                    │                    │                    │─────────┐          │
     │                    │                    │                    │◀────────┘          │
     │                    │                    │                    │                    │
     │  OTLP + BasicAuth  │                    │                    │                    │
     │───────────────────▶│                    │                    │                    │
     │                    │  Authenticate(ctx) │                    │                    │
     │                    │───────────────────▶│                    │                    │
     │                    │                    │  atomic.Load       │                    │
     │                    │                    │  matchFunc(u, p)   │                    │
     │                    │                    │─────────┐          │                    │
     │                    │                    │◀────────┘          │                    │
     │                    │  ctx + AuthData    │                    │                    │
     │                    │◀───────────────────│                    │                    │
     │  200 OK            │                    │                    │                    │
     │◀───────────────────│                    │                    │                    │
     │                    │                    │                    │                    │
     │  OTLP + BadAuth    │                    │                    │                    │
     │───────────────────▶│                    │                    │                    │
     │                    │  Authenticate(ctx) │                    │                    │
     │                    │───────────────────▶│                    │                    │
     │                    │                    │  matchFunc(u, p)   │                    │
     │                    │                    │  → NO MATCH        │                    │
     │                    │  401 Unauthorized  │                    │                    │
     │                    │◀───────────────────│                    │                    │
     │  401 Unauthorized  │                    │                    │                    │
     │◀───────────────────│                    │                    │                    │
```

### Config Provider Flow (Configuration Resolution)

```
Collector Startup    confmap Resolver     Secrets Manager       AWS Secrets Manager
(config loading)     (provider)           Provider              (API)
     │                    │                    │                    │
     │  Resolve config    │                    │                    │
     │───────────────────▶│                    │                    │
     │                    │  Retrieve(uri)     │                    │
     │                    │  "secretsmanager:  │                    │
     │                    │   arn:...:secret"  │                    │
     │                    │───────────────────▶│                    │
     │                    │                    │  GetSecretValue    │
     │                    │                    │───────────────────▶│
     │                    │                    │◀───────────────────│
     │                    │                    │  (extract #key     │
     │                    │                    │   or :-default)    │
     │                    │  confmap.Retrieved │                    │
     │                    │◀───────────────────│                    │
     │  Resolved config   │                    │                    │
     │◀───────────────────│                    │                    │
```

## Behavior

- **Fail-fast**: The extension performs a synchronous initial fetch on startup.
  If it fails, the collector will not start.
- **Error resilience**: On refresh failure, the extension logs an error and
  continues serving with the last successfully fetched credentials.
- **Zero-downtime rotation**: Credentials are stored atomically and read on
  every request, so rotations take effect immediately without dropping connections.
