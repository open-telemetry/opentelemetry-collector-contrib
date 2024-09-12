// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
)

func TestPushLogData(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "http://127.0.0.1:18031"
	config.CreateSchema = false

	err := config.Validate()
	require.NoError(t, err)

	exporter := newLogsExporter(nil, config, testTelemetrySettings)

	ctx := context.Background()

	client, err := createDorisHTTPClient(ctx, config, nil, testTelemetrySettings)
	require.NoError(t, err)
	require.NotNil(t, client)

	exporter.client = client

	defer func() {
		_ = exporter.shutdown(ctx)
	}()

	server := &http.Server{
		ReadTimeout: 5 * time.Second,
	}
	go func() {
		http.HandleFunc("/api/otel/otel_logs/_stream_load", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":"Success"}`))
		})
		server.Addr = ":18031"
		_ = server.ListenAndServe()
	}()

	time.Sleep(1 * time.Second)

	err = exporter.pushLogData(ctx, simpleLogs(10))
	require.NoError(t, err)

	_ = server.Shutdown(ctx)
}

func simpleLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("io.opentelemetry.contrib.doris")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "doris")
	timestamp := time.Now()
	for i := 0; i < count; i++ {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr(semconv.AttributeServiceNamespace, "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}
