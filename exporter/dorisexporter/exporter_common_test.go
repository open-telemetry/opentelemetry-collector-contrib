// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewCommonExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporter, err := newExporter(nil, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)
}

func TestCommonExporter_FormatTime(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exporter, err := newExporter(nil, cfg)
	require.NoError(t, err)
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
	url := streamLoadUrl("http://doris:8030", "otel", "otel_logs")
	require.Equal(t, "http://doris:8030/api/otel/otel_logs/_stream_load", url)
}
