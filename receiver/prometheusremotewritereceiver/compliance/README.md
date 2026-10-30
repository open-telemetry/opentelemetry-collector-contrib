# Prometheus Remote Write Receiver Compliance Tests

This module runs the official [Prometheus Remote Write 2.0 receiver compliance tests](https://github.com/prometheus/compliance/tree/main/remotewrite/receiver) against the OpenTelemetry Collector, using a pipeline with the `prometheus_remote_write` receiver and the `nop` exporter.

## Running

Build the collector binary from the repository root, then run the tests:

```sh
make otelcontribcol
cd receiver/prometheusremotewritereceiver/compliance
go test -v ./
```

To test a specific collector binary, set `OTELCOL_COMPLIANCE_BINARY`:

```sh
OTELCOL_COMPLIANCE_BINARY=/path/to/otelcol go test -v ./
```

Set `DEBUG=1` to stream the collector's output while tests run.
