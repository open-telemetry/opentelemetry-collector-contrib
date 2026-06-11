// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter/internal/metadatatest"
)

// A stream load response may report Status Success while Doris silently drops
// rows (max_filter_ratio > 0). The exporter must surface that partial loss as
// a WARN log and an exporter_doris_filtered_rows counter increment.
func TestPushLogDataFilteredRows(t *testing.T) {
	port, err := findRandomPort()
	require.NoError(t, err)

	config := createDefaultConfig().(*Config)
	config.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.CreateSchema = false
	require.NoError(t, config.Validate())

	tt := componenttest.NewTelemetry()
	defer func() { require.NoError(t, tt.Shutdown(t.Context())) }()

	zapCore, observed := observer.New(zap.WarnLevel)
	logger := zap.New(zapCore)

	set := tt.NewTelemetrySettings()
	exporter, err := newLogsExporter(logger, config, set)
	require.NoError(t, err)

	ctx := t.Context()

	client, err := createDorisHTTPClient(ctx, config, componenttest.NewNopHost(), set)
	require.NoError(t, err)
	require.NotNil(t, client)

	exporter.client = client

	defer func() {
		_ = exporter.shutdown(ctx)
	}()

	srvMux := http.NewServeMux()
	server := &http.Server{
		ReadTimeout: 3 * time.Second,
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     srvMux,
	}

	go func() {
		srvMux.HandleFunc("/api/otel/otel_logs/_stream_load", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":"Success","NumberTotalRows":10,"NumberLoadedRows":7,"NumberFilteredRows":3,"ErrorURL":"http://127.0.0.1:8040/api/_load_error_log?file=error_log"}`))
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

	warns := observed.FilterMessage("doris stream load filtered out rows (partial data loss)").All()
	require.Len(t, warns, 1)
	fields := warns[0].ContextMap()
	assert.EqualValues(t, 3, fields["filtered_rows"])
	assert.EqualValues(t, 7, fields["loaded_rows"])
	assert.Contains(t, fields["error_url"], "_load_error_log")

	metadatatest.AssertEqualExporterDorisFilteredRows(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 3}},
		metricdatatest.IgnoreTimestamp())

	_ = server.Shutdown(ctx)
}

// Loads with no filtered rows must stay silent: no WARN, no counter points.
func TestPushLogDataNoFilteredRowsStaysSilent(t *testing.T) {
	port, err := findRandomPort()
	require.NoError(t, err)

	config := createDefaultConfig().(*Config)
	config.Endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.CreateSchema = false
	require.NoError(t, config.Validate())

	zapCore, observed := observer.New(zap.WarnLevel)
	logger := zap.New(zapCore)

	exporter, err := newLogsExporter(logger, config, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	ctx := t.Context()

	client, err := createDorisHTTPClient(ctx, config, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	exporter.client = client

	defer func() {
		_ = exporter.shutdown(ctx)
	}()

	srvMux := http.NewServeMux()
	server := &http.Server{
		ReadTimeout: 3 * time.Second,
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     srvMux,
	}

	go func() {
		srvMux.HandleFunc("/api/otel/otel_logs/_stream_load", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Status":"Success","NumberTotalRows":10,"NumberLoadedRows":10,"NumberFilteredRows":0}`))
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

	assert.Empty(t, observed.FilterMessage("doris stream load filtered out rows (partial data loss)").All())

	_ = server.Shutdown(ctx)
}
