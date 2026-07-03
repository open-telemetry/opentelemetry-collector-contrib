# Database Authentication Configuration

This package provides the configuration for wiring a **connection-oriented**
component to a `dbauth` provider that supplies a credential at the moment the
component opens a connection — for example, a database receiver that needs a
username and password (or a short-lived token) to dial its database.

It is the configuration half of the `db_auth` framework. The provider
**interface** it resolves (`Provider`, `Credential`, `Request`) lives in a
separate, dependency-light module,
[`extension/dbauth`](../../extension/dbauth), so a provider extension can
implement the interface without taking on this package's confmap dependency

## How a provider is declared and selected

A `dbauth` provider is a Collector **extension** that also implements
`dbauth.Provider`. It is declared once (and listed in `service.extensions`) with
its provider-wide defaults, then each component references it **by component ID**
inside its `db_auth` block — the single key of the block is the provider's
component ID, and its inline value overrides that provider's defaults for this
component only:

```yaml
extensions:
  aws_iam:                       # declared once, with provider-wide defaults
    region: us-east-2

receivers:
  postgresql/this:
    endpoint: this-db:5432
    username: monitor
    db_auth:
      aws_iam:                   # reference the extension by ID
        region: us-east-1        # override a default for this receiver only
  postgresql/another:
    endpoint: another-db:5432
    username: reader
    db_auth:
      aws_iam:                   # no override: use the extension's defaults

service:
  extensions: [aws_iam]
```

The extension holds the provider-wide defaults (here, the AWS region). A
component that needs a different value overrides just that field inline under the
provider ID; a component with no override writes `aws_iam: {}` (or `aws_iam:`)
and inherits the defaults unchanged. The override is merged over the extension's
config **per call**, so the shared extension config is never mutated and
components that differ only by a field need not each declare a named instance.

The **per-connection** inputs a provider needs — the endpoint to connect to, the
database user — are *not* configured on the extension or repeated in the
`db_auth` block. The consuming component (like a database receiver) already
knows them (its own `endpoint` and `username`) and passes them to the provider 
with each `GetCredential` call as a `dbauth.Request`.
So one declared extension serves many
components that differ only by endpoint/user, with no per-database extension
instance and nothing repeated.

This means:

- A provider is **declared once** (one extension in the collector build); any
  receiver can reference it by ID, with no per-receiver provider list to maintain,
  and new providers require no receiver code changes.
- Databases that share provider defaults reference one extension; a database that
  needs a different provider-wide value overrides that field inline under the
  provider ID, with no separate named instance required.

## The credential and the request

The provider interface, `Credential`, and `Request` are defined in
[`extension/dbauth`](../../../extension/dbauth):

```go
type Provider interface {
    GetCredential(ctx context.Context, req Request, extensionArgs map[string]any) (*Credential, error)
}

type Request struct {
    Endpoint string // host:port the credential is for
    Username string // the consumer's configured username
}

type Credential struct {
    Username *string    // nil: use the consumer's configured username
    Secret   string     // opaque: a password or a minted token
    NotAfter *time.Time // nil: no expiry applies; advisory only
}
```

`Request` carries the per-connection inputs the consumer supplies on every call;
a provider uses the fields it needs (AWS IAM mints a token scoped to `Endpoint`
for `Username`) and ignores the rest. Provider-wide configuration (region, a role
to assume) lives on the extension's own config, not in the request.

`extensionArgs` is the inline value the consumer wrote under the provider's ID in
its `db_auth` block — a per-consumer override of the provider's own config. The
provider merges it over its configured defaults for that call only and validates
the result; because the keys are the provider's own config keys, an unrecognized
key (typically a typo) is rejected rather than silently ignored, so a mistyped
override surfaces as an error instead of quietly leaving the default in place. It
is nil or empty when the consumer supplies no override. Because the merge is per
call, a provider must not mutate its own config from `extensionArgs`.

The `Credential` carries credential *material* and nothing about *how* to apply
it — the consuming component decides where the secret goes (for a SQL driver,
usually the password slot of a connection string). The pointer fields are
load-bearing:

- `Username == nil` means the provider supplies no username and the consumer
  should fall back to its own configured username. A non-nil pointer (including a
  pointer to the empty string) means the provider generated the username and the
  consumer must use it. Dynamic providers such as Vault mint both a username and a
  secret per lease, which is why this is a pointer rather than a plain string.
- `NotAfter == nil` means no expiry applies (a static password). When set, it is
  an advisory hint for observability — the provider owns refresh-before-expiry, so
  consumers need not act on it.

## Genericity

The same `Credential` and the same `Provider` interface cover every connection
credential shape without an interface change. How each maps onto the struct:

| Provider               | `Username`        | `Secret`               | `NotAfter`        |
| ---------------------- | ----------------- | ---------------------- | ----------------- |
| Static (inline / file) | from config/file  | the password           | `nil`             |
| AWS IAM (RDS/Aurora)   | `nil`             | minted RDS auth token  | now + ~15m        |
| Azure Managed Identity | `nil`             | minted access token    | token expiry      |
| GCP Cloud SQL IAM      | `nil`             | minted access token    | token expiry      |
| Vault dynamic creds    | minted (non-nil)  | minted password        | lease expiry      |
