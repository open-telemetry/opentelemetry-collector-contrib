// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewTracesExporter(t *testing.T) {

	got, err := newTracesExporter(exportertest.NewNopCreateSettings(), &Config{
		Endpoint: "cn-hangzhou.log.aliyuncs.com",
		Project:  "demo-project",
		Logstore: "demo-logstore",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty()

	// This will put trace data to send buffer and return success.
	err = got.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)
	assert.Nil(t, got.Shutdown(context.Background()))
}

func TestNewFailsWithEmptyTracesExporterName(t *testing.T) {

	got, err := newTracesExporter(exportertest.NewNopCreateSettings(), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
