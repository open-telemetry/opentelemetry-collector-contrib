# PostgreSQL Receiver

This receiver queries the PostgreSQL [statistics collector](https://www.postgresql.org/docs/9.6/monitoring-stats.html).

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver supports PostgreSQL versions 9.6+

Monitoring user must be granted SELECT ON pg_stat_database

## Configuration

The following settings are required to create a database connection:
- `username`
- `password`

The following settings are optional:
- `host` (default = `localhost`): The host of the database. If the `host` field starts with a forward slash (`/`), the [lib/pq](https://github.com/lib/pq) module attempts to use a unix socket connection to `$host.s.PGSQL.$port`, otherwise it will attempt to make a TCP connection.
- `port` (default = `5432`)
- `databases` (default = `[]`): The list of databases for which the receiver will attempt to collect statistics. If an empty list is provided, the receiver will attempt to collect statistics for all non-template databases.
- `ssl_mode` (default = `require`): The SSL mode of the connection. Supported are `require`, `verify-ca`, `verify-full`, `disable`. `ssl_mode` values in the golang [lib/pq](https://github.com/lib/pq) module are modeled after the values [documented here](https://www.postgresql.org/docs/9.6/libpq-ssl.html) for libpq, though there are some minor differences.
- `ssl_root_cert` (default = ""): A set of certificate authorities used to validate the database server's SSL certificate. Only used when `ssl_mode` is not `disable`.
- `ssl_cert` (default = `$HOME/.postgresql/postgresql.crt`): A cerficate used for client authentication, if necessary. Only used when `ssl_mode` is not `disable`.
- `ssl_key` (default = `$HOME/.postgresql/postgresql.key`): A SSL key used to show ownership of `ssl_cert` for client authentication, if necessary. Only used when `ssl_mode` is not `disable`.
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### Example Configuration

```yaml
receivers:
  postgresql:
    host: localhost
    port: 5432
    username: otel
    password: $POSTGRESQL_PASSWORD
    databases:
      - otel
    collection_interval: 10s
    ssl_mode: verify-ca
    ssl_root_cert: /home/otel/authorities.crt
    ssl_cert: /home/otel/mypostgrescert.crt
    ssl_key: /home/otel/mypostgreskey.key
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)
