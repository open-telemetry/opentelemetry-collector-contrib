# Redis Receiver

The Redis receiver is designed to retrieve metadata from a single Redis instance,
build metrics from that data, and send them to the next consumer at a configurable interval.

# Configuration

Example configuration:

```yaml
receivers:
  redis:
    endpoint: "localhost:6379"
    refresh_interval: "10s"
    password: "s3cret"
```

### endpoint

The hostname and port of the Redis instance, separated by a colon.

_Required._

### refresh_interval

This receiver runs on an interval. Each time it runs, it queries Redis, creates
metrics, and send them to the next consumer. The `refresh_interval` configuration
option tells this receiver the interval duration between runs.

Must be a string value that is readble by Golang's `ParseDuration` function. e.g. "1h30m".
Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

_Required._

### password

The password used to access the Redis instance. Must match the password specified in
the requirepass server configuration option.

_Optional._
