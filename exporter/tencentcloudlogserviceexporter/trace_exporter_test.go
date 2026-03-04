// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func TestNewTracesExporter(t *testing.T) {
	got, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), &Config{
		Region:    "ap-beijing",
		LogSet:    "demo-logset",
		Topic:     "demo-topic",
		SecretKey: "123456",
		SecretID:  "123456",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty()

	// This will put trace data to send buffer and return success.
	err = got.ConsumeTraces(t.Context(), traces)
	assert.NoError(t, err)
	assert.NoError(t, got.Shutdown(t.Context()))
}

func TestNewFailsWithEmptyTracesExporterName(t *testing.T) {
	got, err := newTracesExporter(exportertest.NewNopSettings(metadata.Type), &Config{
		Region:    "ap-beijing",
		LogSet:    "demo-logset",
		Topic:     "demo-topic",
		SecretID:  "access-id",
		SecretKey: "secret-key",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)
}
