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

	"github.com/prometheus/compliance/remotewrite/sender"
	"github.com/stretchr/testify/require"
)

const collectorConfigTemplate = `
receivers:
  prometheus:
    discovery_reload_on_startup: true
    config:
      scrape_configs:
        - job_name: "{{.ScrapeTargetJobName}}"
          scrape_interval: 1s
          scrape_native_histograms: true
          scrape_protocols:
            - PrometheusProto
            - OpenMetricsText1.0.0
            - PrometheusText0.0.4
          static_configs:
            - targets: ["{{.ScrapeTargetHostPort}}"]

exporters:
  prometheusremotewrite:
    endpoint: "{{.RemoteWriteEndpointURL}}"
    protobuf_message: "{{.RemoteWriteMessage}}"
    send_metadata: true
    translation_strategy: UnderscoreEscapingWithoutSuffixes
{{- if eq (printf "%s" .RemoteWriteMessage) "io.prometheus.write.v2.Request"}}
    convert_explicit_histograms_to_nhcb: true
{{- end}}
    remote_write_queue:
      num_consumers: 1

service:
  telemetry:
    logs:
      level: error
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [prometheus]
      exporters: [prometheusremotewrite]
`

var collectorConfigTmpl = template.Must(template.New("config").Parse(collectorConfigTemplate))

type otelCollector struct {
	binary string
}

func (otelCollector) Name() string { return "otelcol-contrib" }

// Run runs the collector binary as a test sender target, until ctx is done.
func (c otelCollector) Run(ctx context.Context, opts sender.Options) error {
	var buf bytes.Buffer
	if err := collectorConfigTmpl.Execute(&buf, opts); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	dir, err := os.MkdirTemp("", "rw-compliance-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, buf.Bytes(), 0o600); err != nil {
		return err
	}

	featureGates := []string{
		// Required for the exporter to accept protobuf_message: io.prometheus.write.v2.Request.
		"exporter.prometheusremotewritexporter.enableSendingRW2",
		// Required to retry HTTP 429 responses.
		"exporter.prometheusremotewritexporter.RetryOn429",
		// Propagate start (created) timestamps from scraped _created lines.
		"receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion",
	}
	return sender.RunCommand(ctx, dir, nil, c.binary,
		fmt.Sprintf("--config=%s", configFile),
		fmt.Sprintf("--feature-gates=%s", strings.Join(featureGates, ",")),
	)
}

var _ sender.Sender = otelCollector{}

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

	// Skip known compliance gaps:
	// - samples_sorted: receiver/prometheus deduplicates multiple samples for the same series in one scrape.
	// - start_timestamp_*: receiver/prometheus truncates float64 _created timestamps by up to 1ms.
	// - float_histogram: pmetric exponential histograms store bucket counts as uint64.
	// - type_info_and_stateset: OpenTelemetry does not support INFO/STATESET, those are gauges.
	skip := strings.Join([]string{
		"TestRemoteWriteSender/otelcol-contrib/samples_sorted",
		"TestRemoteWriteSender/otelcol-contrib/samples/rw2/start_timestamp_for_.*",
		"TestRemoteWriteSender/otelcol-contrib/native_histograms/rw2/(float_histogram|start_timestamp_before_timestamp)",
		"TestRemoteWriteSender/otelcol-contrib/metadata/rw2/type_info_and_stateset",
	}, "|")
	if existing := flag.Lookup("test.skip").Value.String(); existing != "" {
		skip = existing + "|" + skip
	}
	_ = flag.Set("test.skip", skip)

	os.Exit(m.Run())
}

// TestRemoteWriteSender runs the remote write sender compliance tests defined
// in https://github.com/prometheus/compliance/tree/main/remotewrite/sender.
func TestRemoteWriteSender(t *testing.T) {
	bin := collectorBinary(t)
	sender.RunTests(t, otelCollector{binary: bin}, sender.ComplianceTests())
}
