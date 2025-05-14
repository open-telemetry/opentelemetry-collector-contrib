// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func createSimpleLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("mylog")
		logRecord.Attributes().PutStr(string(conventions.ServiceNameKey), "myapp")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr(string(conventions.HostNameKey), "myhost")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}
	sl.LogRecords().AppendEmpty()

	return logs
}

func TestNewLogsExporter(t *testing.T) {
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{
		Region: "ap-beijing",
		LogSet: "demo-logset",
		Topic:  "demo-topic",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This will put log data to send buffer and return success.
	// Tip: Unit tests require valid access credential.
	_ = got.ConsumeLogs(context.Background(), createSimpleLogData(3))
}

func TestNewFailsWithEmptyLogsExporterName(t *testing.T) {
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{})
	assert.NoError(t, err)
	require.NotNil(t, got)
}
