// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package compliance

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/prometheus/compliance/remotewrite/receiver"
	"github.com/stretchr/testify/require"
)

const collectorConfigTemplate = `
receivers:
  prometheus_remote_write:
    endpoint: "{{.ReceiveHostPort}}"

exporters:
  nop:

service:
  telemetry:
    logs:
      level: error
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [prometheus_remote_write]
      exporters: [nop]
`

var collectorConfigTmpl = template.Must(template.New("config").Parse(collectorConfigTemplate))

type otelCollector struct {
	binary string
}

func (otelCollector) Name() string { return "otelcol-contrib" }

// Run runs the collector binary as a test receiver target, until ctx is done.
func (c otelCollector) Run(ctx context.Context, opts receiver.Options) error {
	var buf bytes.Buffer
	if err := collectorConfigTmpl.Execute(&buf, opts); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	dir, err := os.MkdirTemp("", "rw-receiver-compliance-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, buf.Bytes(), 0o600); err != nil {
		return err
	}

	return receiver.RunCommand(ctx, dir, nil, c.binary,
		fmt.Sprintf("--config=%s", configFile),
	)
}

var _ receiver.Receiver = otelCollector{}

// collectorBinary locates the collector binary to test. It defaults to the
// output of `make otelcontribcol`, overridable via OTELCOL_COMPLIANCE_BINARY.
func collectorBinary(tb testing.TB) string {
	tb.Helper()

	if path := os.Getenv("OTELCOL_COMPLIANCE_BINARY"); path != "" {
		abs, err := filepath.Abs(path)
		require.NoError(tb, err)
		_, err = os.Stat(abs)
		require.NoErrorf(tb, err, "collector binary not found at %s (set via OTELCOL_COMPLIANCE_BINARY)", abs)
		return abs
	}

	path := filepath.Join("../../../bin", collectorBinaryName(runtime.GOOS, runtime.GOARCH))
	abs, err := filepath.Abs(path)
	require.NoError(tb, err)
	if _, err = os.Stat(abs); err != nil {
		tb.Skipf("skipping compliance tests: collector binary not found at %s; build it with `make otelcontribcol` or set OTELCOL_COMPLIANCE_BINARY", abs)
	}
	return abs
}

func collectorBinaryName(goos, goarch string) string {
	name := fmt.Sprintf("otelcontribcol_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Skip known compliance gaps in receiver/prometheusremotewritereceiver:
	// - out_of_order_rejected: the receiver is a stateless translator and does not track series timestamps across requests.
	// - invalid_utf8_*: symbol table entries are not validated for UTF-8 before constructing labels.
	// - bad_count_*, reset_hint_gauge_*, invalid_rejected/*: invalid or gauge histograms are dropped with 204 instead of rejected with 400.
	// - exemplars/validation/*: invalid exemplar label refs are dropped instead of rejected with 400.
	// - summary_*: summaries are intentionally unsupported and dropped without incrementing Samples-Written.
	// - metadata_without_data: series with only metadata and no data points return 204 instead of 400.
	// - request/* and modes/content_type/rfc9110_*: HTTP method, Content-Encoding (confighttp), Accept/Accept-Encoding 415 headers, and RFC 9110 mode params are not strictly enforced.
	// - rw1_compat/basic_compatibility: Remote Write 1.0 is intentionally unsupported.
	// - modes/validation/*: stateless and stateless-st mode SHOULD rejections are not implemented.
	skip := strings.Join([]string{
		"TestRemoteWriteReceiver/otelcol-contrib/samples/out_of_order_rejected",
		"TestRemoteWriteReceiver/otelcol-contrib/symbols/invalid_utf8_.*",
		"TestRemoteWriteReceiver/otelcol-contrib/histograms/bad_count_histogram",
		"TestRemoteWriteReceiver/otelcol-contrib/histograms/float/.*",
		"TestRemoteWriteReceiver/otelcol-contrib/histograms/reset_hints_and_schemas/reset_hint_gauge_schema_5",
		"TestRemoteWriteReceiver/otelcol-contrib/histograms/invalid_rejected/.*",
		"TestRemoteWriteReceiver/otelcol-contrib/exemplars/validation/.*",
		"TestRemoteWriteReceiver/otelcol-contrib/metadata/summary_metadata_type",
		"TestRemoteWriteReceiver/otelcol-contrib/combined/edge_cases/metadata_without_data",
		"TestRemoteWriteReceiver/otelcol-contrib/request/(bad_content_type_modes|unsupported_content_type_accept_header|bad_content_encodings|unsupported_content_encoding_accept_encoding_header|snappy_framed_rejected|unsupported_http_methods)",
		"TestRemoteWriteReceiver/otelcol-contrib/rw1_compat/basic_compatibility",
		"TestRemoteWriteReceiver/otelcol-contrib/modes/content_type/rfc9110_(uppercase_params|quoted_values)",
		"TestRemoteWriteReceiver/otelcol-contrib/modes/stateless/summary_series_with_metadata",
		"TestRemoteWriteReceiver/otelcol-contrib/modes/stateless_st/(summary_with_start_timestamp_on_sum_and_count|float_histogram_with_start_timestamp)",
		"TestRemoteWriteReceiver/otelcol-contrib/modes/validation/.*",
	}, "|")
	if existing := flag.Lookup("test.skip").Value.String(); existing != "" {
		skip = existing + "|" + skip
	}
	_ = flag.Set("test.skip", skip)

	os.Exit(m.Run())
}

// TestRemoteWriteReceiver runs the remote write receiver compliance tests defined
// in https://github.com/prometheus/compliance/tree/main/remotewrite/receiver.
func TestRemoteWriteReceiver(t *testing.T) {
	bin := collectorBinary(t)
	receiver.RunTests(t, otelCollector{binary: bin}, receiver.ComplianceTests())
}
