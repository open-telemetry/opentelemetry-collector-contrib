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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func Test_NewLogsExporter(t *testing.T) {

	type args struct {
		config *Config
		logger *zap.Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"NewLogExporter: success",
			args{
				config: &Config{
					ExporterSettings:    config.NewExporterSettings(component.NewID("logicmonitor")),
					URL:                 "https://test.logicmonitor.com/rest",
					APIToken:            map[string]string{"access_id": "testid", "access_key": "testkey"},
					LogBatchingEnabled:  true,
					LogBatchingInterval: 50 * time.Millisecond,
				},
				logger: zap.NewNop(),
			},
			false,
		},
		{
			"NewLogExporter: fail",
			args{
				config: &Config{
					ExporterSettings:    config.NewExporterSettings(component.NewID("logicmonitor")),
					URL:                 "https://test.logicmonitor.com/rest",
					APIToken:            map[string]string{"access_id": "testid", "access_key": "testkey"},
					LogBatchingEnabled:  true,
					LogBatchingInterval: 20 * time.Millisecond,
				},
				logger: zap.NewNop(),
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			_, err := newLogsExporter(tt.args.config, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("newLogsExporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestPushLogData(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		response := Response{
			Success: true,
			Message: "Logs exported successfully",
		}
		body, _ := json.Marshal(response)
		_, _ = w.Write(body)
	}))

	type args struct {
		ctx context.Context
		lg  plog.Logs
	}

	type fields struct {
		batchEnabled bool
		config       *Config
		logger       *zap.Logger
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Send Log data",
			fields: fields{
				batchEnabled: false,
				logger:       zap.NewNop(),
				config:       cfg,
			},
			args: args{
				ctx: context.Background(),
				lg:  createLogData(1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			var options []logs.Option
			if test.fields.batchEnabled {
				options = []logs.Option{
					logs.WithLogBatchingInterval(1 * time.Second),
				}
			}
			client, err := logs.NewLMLogIngest(context.Background(), options...)
			fmt.Println("Error while creating log ingest client: ", err)
			e := &logExporter{
				logger:          test.fields.logger,
				config:          test.fields.config,
				logIngestClient: client,
			}
			err = e.PushLogData(test.args.ctx, test.args.lg)
			if err != nil {
				t.Errorf("logicmonitorexporter.PushLogsData() error = %v", err)
				return
			}
		})
	}
}

func createLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "myapp")
	rl.Resource().Attributes().PutInt("http.statusCode", 200)
	rl.Resource().Attributes().PutBool("isTest", true)
	rl.Resource().Attributes().PutDouble("value", 20.00)
	rl.Resource().Attributes().PutEmptySlice("values")
	rl.Resource().Attributes().PutEmptyMap("valueMap")
	rl.ScopeLogs().AppendEmpty() // Add an empty InstrumentationLibraryLogs
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
