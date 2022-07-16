// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logzioexporter

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Logs
func GenerateLogRecordWithNestedBody() plog.LogRecord {
	lr := plog.NewLogRecord()
	fillLogOne(lr)
	return lr
}
func GenerateLogRecordWithMultiTypeValues() plog.LogRecord {
	lr := plog.NewLogRecord()
	fillLogTwo(lr)
	return lr
}

func TestConvertLogRecordToJSON(t *testing.T) {
	logger := hclog.NewNullLogger()
	type convertLogRecordToJSONTest struct {
		log      plog.LogRecord
		resource pcommon.Resource
		expected map[string]interface{}
	}

	var convertLogRecordToJSONTests = []convertLogRecordToJSONTest{
		{GenerateLogRecordWithNestedBody(),
			pcommon.NewResource(),
			map[string]interface{}{
				"23":           float64(45),
				"app":          "server",
				"foo":          "bar",
				"instance_num": float64(1),
				"level":        "Info",
				"message":      "hello there",
				"@timestamp":   TestLogTimeUnixMilli,
				"nested":       map[string]interface{}{"number": float64(499), "string": "v1"},
				"spanID":       "0102040800000000",
				"traceID":      "08040201000000000000000000000000",
			},
		},
		{GenerateLogRecordWithMultiTypeValues(),
			pcommon.NewResource(),
			map[string]interface{}{
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
		output := convertLogRecordToJSON(test.log, test.resource, logger)
		require.Equal(t, output, test.expected)
	}

}
func TestSetTimeStamp(t *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = ioutil.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	ld := GenerateLogsOneEmptyTimestamp()
	cfg := &Config{
		Region:           "us",
		Token:            "token",
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:    server.URL,
			Compression: configcompression.Gzip,
		},
	}
	var err error
	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := createLogsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()
	err = exporter.ConsumeLogs(ctx, ld)
	require.NoError(t, err)
	err = exporter.Shutdown(ctx)
	require.NoError(t, err)
	var jsonLog map[string]interface{}
	var jsonLogNoTimestamp map[string]interface{}
	decoded, _ := gUnzipData(recordedRequests)
	requests := strings.Split(string(decoded), "\n")
	require.NoError(t, json.Unmarshal([]byte(requests[0]), &jsonLog))
	require.NoError(t, json.Unmarshal([]byte(requests[1]), &jsonLogNoTimestamp))
	if jsonLogNoTimestamp["@timestamp"] != nil {
		t.Fatalf("did not expect @timestamp")
	}
	if jsonLog["@timestamp"] == nil {
		t.Fatalf("@timestamp does not exist")
	}
}
