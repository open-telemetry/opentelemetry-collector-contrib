# Chrony Receiver

| Status                   |           |
| ------------------------ | --------- |
| Stability                | [alpha]   |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The [chrony] receiver is a pure go implementation of the command `chronyc tracking` to allow for
portability across systems and platforms. All of the data that would typically be captured by
the tracking command is made available in this receiver, see [documentation](./documentation.md) for
more details.

## Configuration

### Default

By default, the `chrony` receiver will default to the following configuration:

```yaml
chrony/defaults:
  address: unix:///var/run/chrony/chronyd.sock # The default port by chronyd to allow cmd access
  timeout: 10s # Allowing at least 10s for chronyd to respond before giving up

chrony:
  # This will result in the same configuration as above
```

### Customised

The following options can be customised:

- address (required) - the address on where to communicate to `chronyd`
  - The allowed formats are the following
    - udp://hostname:port
    - unix:///path/to/chrony.sock (Please note the triple slash)
- timeout (optional) - The total amount of time allowed to read and process the data from chronyd
  - Recommendation: This value should be set above 1s to allow `chronyd` time to respond
- collection_interval (optional) - how frequent this receiver should poll [chrony]
- metrics (optional) - Which metrics should be exported, read the [documentation] for complete details

## Example

An example of the configuration is:

```yaml
receivers:
  chrony:
    address: unix:///var/run/chrony/chronyd.sock
    timeout: 10s
    collection_interval: 30s
    metrics:
      ntp.skew:
        enabled: true
      ntp.stratum:
        enabled: true
```

The complete list of metrics emitted by this receiver is found in the [documentation].

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[documentation]: ./documentation.md
[chrony]: https://chrony.tuxfamily.org/
