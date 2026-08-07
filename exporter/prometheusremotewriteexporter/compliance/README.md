# Prometheus Remote Write Sender Compliance Tests

This module runs the official [Prometheus Remote Write sender compliance
tests](https://github.com/prometheus/compliance/tree/main/remotewrite/sender)
against the OpenTelemetry Collector, using a pipeline made of the
`prometheus` receiver and the `prometheusremotewrite` exporter.

The test harness starts a mock scrape target and a mock remote write
receiver, launches a collector binary that scrapes the target and remote
writes to the receiver, and validates the requests the receiver captures.

## Running

Build the collector binary first, then run the tests:

```sh
make otelcontribcol # from the repository root
cd exporter/prometheusremotewriteexporter/compliance
go test -v ./
```

To test a different binary, set `OTELCOL_COMPLIANCE_BINARY`:

```sh
OTELCOL_COMPLIANCE_BINARY=/path/to/otelcol go test -v ./
```

Set `DEBUG=1` to see the collector's output while tests run.

These tests run in CI via the `prometheus-compliance-tests` workflow. Tests
that fail due to known compliance gaps are skipped there; see the workflow
file for details.
