// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/otel/metric"
)

var testTelemetrySettings = component.TelemetrySettings{
	LeveledMeterProvider: func(_ configtelemetry.Level) metric.MeterProvider {
		return nil
	},
}

func TestNewCommonExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporter := newExporter(nil, cfg, testTelemetrySettings)
	require.NotNil(t, exporter)
}

func TestCommonExporter_FormatTime(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporter := newExporter(nil, cfg, testTelemetrySettings)
	require.NotNil(t, exporter)

	now := time.Date(2024, 1, 1, 0, 0, 0, 1000, time.Local)
	require.Equal(t, "2024-01-01 00:00:00.000001", exporter.formatTime(now))
}

func TestStreamLoadResponse_Success(t *testing.T) {
	resp := &streamLoadResponse{
		Status: "Success",
	}
	require.True(t, resp.success())

	resp.Status = "Publish Timeout"
	require.True(t, resp.success())

	resp.Status = "Fail"
	require.False(t, resp.success())
}

func TestStreamLoadUrl(t *testing.T) {
	url := streamLoadURL("http://doris:8030", "otel", "otel_logs")
	require.Equal(t, "http://doris:8030/api/otel/otel_logs/_stream_load", url)
}

func findRandomPort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")

	if err != nil {
		return 0, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	err = l.Close()

	if err != nil {
		return 0, err
	}

	return port, nil
}
