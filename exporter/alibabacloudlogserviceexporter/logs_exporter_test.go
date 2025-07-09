// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter/internal/metadata"
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
		Endpoint: "us-west-1.log.aliyuncs.com",
		Project:  "demo-project",
		Logstore: "demo-logstore",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This will put trace data to send buffer and return success.
	err = got.ConsumeLogs(context.Background(), createSimpleLogData(3))
	assert.NoError(t, err)
	time.Sleep(time.Second * 4)
}

func TestSTSTokenExporter(t *testing.T) {
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{
		Endpoint:      "us-west-1.log.aliyuncs.com",
		Project:       "demo-project",
		Logstore:      "demo-logstore",
		TokenFilePath: filepath.Join("testdata", "config.yaml"),
	})
	assert.NoError(t, err)
	require.NotNil(t, got)
}

func TestNewFailsWithEmptyLogsExporterName(t *testing.T) {
	got, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
