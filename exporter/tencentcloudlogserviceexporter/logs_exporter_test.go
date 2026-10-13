// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func createSimpleLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	for i := range numberOfLogs {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("mylog")
		logRecord.Attributes().PutStr("service.name", "myapp")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr("host.name", "myhost")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}
	sl.LogRecords().AppendEmpty()

	return logs
}

func TestNewLogsExporter(t *testing.T) {
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{
		Region:    "ap-beijing",
		LogSet:    "demo-logset",
		Topic:     "demo-topic",
		SecretID:  "access-id",
		SecretKey: "secret-key",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This will put log data to send buffer and return success.
	// Tip: Unit tests require valid access credential.
	_ = got.ConsumeLogs(t.Context(), createSimpleLogData(3))

	// Shutdown flushes the buffered logs and stops the SDK background workers.
	assert.NoError(t, got.Shutdown(t.Context()))
}

func TestNewLogsExporterFailsWithEmptyConfig(t *testing.T) {
	// The CLS SDK rejects a producer without credentials, so an empty config is an error
	// rather than a silently unusable exporter.
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
