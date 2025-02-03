// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Logs
func generateLogRecordWithNestedBody() plog.LogRecord {
	lr := plog.NewLogRecord()
	fillLogOne(lr)
	return lr
}

func generateLogRecordWithMultiTypeValues() plog.LogRecord {
	lr := plog.NewLogRecord()
	fillLogTwo(lr)
	return lr
}

func TestConvertLogRecordToJSON(t *testing.T) {
	type convertLogRecordToJSONTest struct {
		log      plog.LogRecord
		resource pcommon.Resource
		expected map[string]any
	}

	convertLogRecordToJSONTests := []convertLogRecordToJSONTest{
		{
			generateLogRecordWithNestedBody(),
			pcommon.NewResource(),
			map[string]any{
				"23":           float64(45),
				"app":          "server",
				"foo":          "bar",
				"instance_num": float64(1),
				"level":        "Info",
				"message":      "hello there",
				"@timestamp":   TestLogTimeUnixMilli,
				"nested":       map[string]any{"number": float64(499), "string": "v1"},
				"spanID":       "0102040800000000",
				"traceID":      "08040201000000000000000000000000",
			},
		},
		{
			generateLogRecordWithMultiTypeValues(),
			pcommon.NewResource(),
			map[string]any{
				"bool":       true,
				"customer":   "acme",
				"env":        "dev",
				"level":      "Info",
				"@timestamp": TestLogTimeUnixMilli,
				"message":    "something happened",
				"number":     float64(64),
			},
		},
	}
	for _, test := range convertLogRecordToJSONTests {
		output := convertLogRecordToJSON(test.log, test.log.Attributes())
		require.Equal(t, test.expected, output)
	}
}

func TestSetTimeStamp(t *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = io.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()
	ld := generateLogsOneEmptyTimestamp()
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	clientConfig.Compression = configcompression.TypeGzip
	cfg := &Config{
		Region:       "us",
		Token:        "token",
		ClientConfig: clientConfig,
	}
	var err error
	params := exportertest.NewNopSettings()
	exporter, err := createLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()
	err = exporter.ConsumeLogs(ctx, ld)
	require.NoError(t, err)
	err = exporter.Shutdown(ctx)
	require.NoError(t, err)
	var jsonLog map[string]any
	var jsonLogNoTimestamp map[string]any
	decoded, _ := gUnzipData(recordedRequests)
	requests := strings.Split(string(decoded), "\n")
	require.NoError(t, json.Unmarshal([]byte(requests[0]), &jsonLog))
	require.NoError(t, json.Unmarshal([]byte(requests[1]), &jsonLogNoTimestamp))
	require.Nil(t, jsonLogNoTimestamp["@timestamp"], "did not expect @timestamp")
	require.NotNil(t, jsonLog["@timestamp"], "@timestamp does not exist")
}
