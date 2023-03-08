// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type logsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func Test_NewLogsExporter(t *testing.T) {
	tests := []struct {
		name string
		args struct {
			config *Config
			logger *zap.Logger
		}
	}{
		{
			name: "Create NewLogExporter",
			args: struct {
				config *Config
				logger *zap.Logger
			}{
				config: &Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "http://example.logicmonitor.com/rest",
					},
					APIToken: APIToken{AccessID: "testid", AccessKey: "testkey"},
				},
				logger: zaptest.NewLogger(t),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := exportertest.NewNopCreateSettings()
			exp := newLogsExporter(tt.args.config, set)
			assert.NotNil(t, exp)
		})
	}
}

func TestPushLogData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := logsResponse{
			Success: true,
			Message: "Logs exported successfully",
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))
	defer ts.Close()

	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: ts.URL,
		},
		APIToken: APIToken{AccessID: "testid", AccessKey: "testkey"},
	}

	tests := []struct {
		name   string
		fields struct {
			config *Config
			logger *zap.Logger
		}
		args struct {
			ctx context.Context
			lg  plog.Logs
		}
	}{
		{
			name: "Send Log data",
			fields: struct {
				config *Config
				logger *zap.Logger
			}{
				logger: zaptest.NewLogger(t),
				config: cfg,
			},
			args: struct {
				ctx context.Context
				lg  plog.Logs
			}{
				ctx: context.Background(),
				lg:  createLogData(1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set := exportertest.NewNopCreateSettings()
			exp := newLogsExporter(test.fields.config, set)

			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
			err := exp.PushLogData(test.args.ctx, test.args.lg)
			assert.NoError(t, err)
		})
	}
}

func createLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "myapp")
	rl.Resource().Attributes().PutStr("hostname", "myapp")
	rl.Resource().Attributes().PutInt("http.statusCode", 200)
	rl.Resource().Attributes().PutBool("isTest", true)
	rl.Resource().Attributes().PutDouble("value", 20.00)
	rl.Resource().Attributes().PutEmptySlice("values")
	rl.Resource().Attributes().PutEmptyMap("valueMap")
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	ill := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := ill.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("mylog")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}
	ill.LogRecords().AppendEmpty()

	return logs
}
