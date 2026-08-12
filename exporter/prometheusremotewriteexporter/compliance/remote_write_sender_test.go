// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package compliance runs the official Prometheus Remote Write sender
// compliance tests (https://github.com/prometheus/compliance/tree/main/remotewrite/sender)
// against the OpenTelemetry Collector's prometheusremotewrite exporter.
//
// The tests scrape a mock target and validate the requests the collector
// sends to a mock remote write receiver. They require a collector binary;
// build one with `make otelcontribcol` from the repository root, or point
// OTELCOL_COMPLIANCE_BINARY at an existing binary.
package compliance

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/prometheus/compliance/remotewrite/sender"
)

const collectorConfigTemplate = `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: "{{.ScrapeTargetJobName}}"
          scrape_interval: 1s
          scrape_protocols:
            - OpenMetricsText1.0.0
          static_configs:
            - targets: ["{{.ScrapeTargetHostPort}}"]

exporters:
  prometheusremotewrite:
    endpoint: "{{.RemoteWriteEndpointURL}}"
    protobuf_message: "{{.RemoteWriteMessage}}"
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

type otelCollector struct{}

func (otelCollector) Name() string { return "otelcol-contrib" }

// Run runs the collector binary as a test sender target, until ctx is done.
func (otelCollector) Run(ctx context.Context, opts sender.Options) error {
	binary, err := collectorBinary()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err = collectorConfigTmpl.Execute(&buf, opts); err != nil {
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
		// Propagate start (created) timestamps from scraped _created lines.
		"receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion",
	}
	return sender.RunCommand(ctx, dir, nil, binary,
		fmt.Sprintf("--config=%s", configFile),
		fmt.Sprintf("--feature-gates=%s", strings.Join(featureGates, ",")),
	)
}

var _ sender.Sender = otelCollector{}

// collectorBinary locates the collector binary to test. It defaults to the
// output of `make otelcontribcol`, overridable via OTELCOL_COMPLIANCE_BINARY.
func collectorBinary() (string, error) {
	path := os.Getenv("OTELCOL_COMPLIANCE_BINARY")
	if path == "" {
		path = filepath.Join("../../../bin", collectorBinaryName(runtime.GOOS, runtime.GOARCH))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("collector binary not found at %s; build it with `make otelcontribcol` or set OTELCOL_COMPLIANCE_BINARY: %w", abs, err)
	}
	return abs, nil
}

func collectorBinaryName(goos, goarch string) string {
	name := fmt.Sprintf("otelcontribcol_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// TestRemoteWriteSender runs the remote write sender compliance tests defined
// in https://github.com/prometheus/compliance/tree/main/remotewrite/sender.
func TestRemoteWriteSender(t *testing.T) {
	sender.RunTests(t, otelCollector{}, sender.ComplianceTests())
}
