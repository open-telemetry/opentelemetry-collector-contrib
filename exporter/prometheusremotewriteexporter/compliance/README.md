# Prometheus Remote Write Sender Compliance Tests

This module runs the official [Prometheus Remote Write sender compliance tests](https://github.com/prometheus/compliance/tree/main/remotewrite/sender) against the OpenTelemetry Collector, using a pipeline with the `prometheus` receiver and the `prometheusremotewrite` exporter.

## Running

Build the collector binary from the repository root, then run the tests:

```sh
make otelcontribcol
cd exporter/prometheusremotewriteexporter/compliance
go test -v ./
```

To test a specific collector binary, set `OTELCOL_COMPLIANCE_BINARY`:

```sh
OTELCOL_COMPLIANCE_BINARY=/path/to/otelcol go test -v ./
```

Set `DEBUG=1` to stream the collector's output while tests run.
