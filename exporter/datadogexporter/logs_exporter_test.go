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

package datadogexporter

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func Test_logs_exporter_send_logs(t *testing.T) {
	lr := testdata.GenerateLogsOneLogRecord()
	ld := lr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	type args struct {
		sendLogRecordBody bool
		ld                plog.Logs
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "log_with_out_message",
			args: args{
				sendLogRecordBody: false,
				ld:                lr,
			},
			want: func() string {
				return fmt.Sprintf(`
            [{
                "message": "",
                "app" : "server",
                "instance_num": "1",
                "@timestamp" :"%s" ,
                "status" : "Info",
                "dd.span_id" :"%d",
                "dd.trace_id" : "%d",
                "otel.severity_text":"Info",
                "otel.severity_number":"9",
                "otel.span_id":"%s",
                "otel.trace_id" : "%s",
                "otel.timestamp" : "%d"

            }]
            `, testdata.TestLogTime.Format(time.RFC3339), spanIDToUint64(ld.SpanID()), traceIDToUint64(ld.TraceID()), ld.SpanID().HexString(), ld.TraceID().HexString(), testdata.TestLogTime.UnixNano())
			}(),
		},
		{
			name: "log_with_message",
			args: args{
				sendLogRecordBody: true,
				ld:                lr,
			},
			want: func() string {
				return fmt.Sprintf(`
            [{
                "message": "%s",
                "app" : "server",
                "instance_num": "1",
                "@timestamp" :"%s" ,
                "status" : "Info",
                "dd.span_id" :"%d",
                "dd.trace_id" : "%d",
                "otel.severity_text":"Info",
                "otel.severity_number":"9",
                "otel.span_id":"%s",
                "otel.trace_id" : "%s",
                "otel.timestamp" : "%d"

            }]
            `, ld.Body().AsString(), testdata.TestLogTime.Format(time.RFC3339), spanIDToUint64(ld.SpanID()), traceIDToUint64(ld.TraceID()), ld.SpanID().HexString(), ld.TraceID().HexString(), testdata.TestLogTime.UnixNano())
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := testutils.DatadogLogServerMock()
			defer server.Close()
			cfg := &Config{
				Metrics: MetricsConfig{
					TCPAddr: confignet.TCPAddr{
						Endpoint: server.URL,
					},
				},
				Logs: LogsConfig{
					TCPAddr: confignet.TCPAddr{
						Endpoint: server.URL,
					},
					SendLogRecordBody: tt.args.sendLogRecordBody,
				},
			}

			params := componenttest.NewNopExporterCreateSettings()
			f := NewFactory()
			ctx := context.Background()
			exp, err := f.CreateLogsExporter(ctx, params, cfg)
			require.NoError(t, err)
			require.NoError(t, exp.ConsumeLogs(ctx, tt.args.ld))
			var want []map[string]interface{}
			err = json.Unmarshal([]byte(tt.want), &want)
			if err != nil {
				t.Fatalf("error=%v", err)
				return
			}
			assert.Equal(t, want, server.LogsData)
		})
	}

}

// traceIDToUint64 converts 128bit traceId to 64 bit uint64
func traceIDToUint64(b [16]byte) uint64 {
	return binary.BigEndian.Uint64(b[len(b)-8:])
}

// spanIDToUint64 converts byte array to uint64
func spanIDToUint64(b [8]byte) uint64 {
	return binary.BigEndian.Uint64(b[:])
}
