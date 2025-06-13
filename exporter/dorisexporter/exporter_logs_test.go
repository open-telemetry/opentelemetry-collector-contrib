// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"
)

func TestPushLogData(t *testing.T) {
	port, err := findRandomPort()
	require.NoError(t, err)

	config := createDefaultConfig().(*Config)
	config.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.CreateSchema = false

	err = config.Validate()
	require.NoError(t, err)

	logger := zap.NewNop()
	exporter := newLogsExporter(logger, config, componenttest.NewNopTelemetrySettings())

	ctx := context.Background()

	client, err := createDorisHTTPClient(ctx, config, nil, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, client)

	exporter.client = client

	defer func() {
		_ = exporter.shutdown(ctx)
	}()

	server := &http.Server{
		ReadTimeout: 3 * time.Second,
		Addr:        fmt.Sprintf(":%d", port),
	}

	go func() {
		http.HandleFunc("/api/otel/otel_logs/_stream_load", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":"Success"}`))
		})
		err = server.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()

	err0 := errors.New("Not Started")
	for i := 0; err0 != nil && i < 10; i++ { // until server started
		err0 = exporter.pushLogData(ctx, simpleLogs(10))
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err0)

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
