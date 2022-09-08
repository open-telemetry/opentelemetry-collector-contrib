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

package logs

import (
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestTransform(t *testing.T) {
	type args struct {
		lr  plog.LogRecord
		res pcommon.Resource
	}
	tests := []struct {
		name string
		args args
		want datadogV2.HTTPLogItem
	}{
		{
			name: "log_with_attribute",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().InsertString("app", "test")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: pcommon.NewResource(),
			},
			want: datadogV2.HTTPLogItem{
				Message: *datadog.PtrString(""),
				AdditionalProperties: map[string]string{
					"app":                   "test",
					"status":                "debug",
					"otel.serverity_number": "5",
				},
			},
		},
		{
			name: "log_and_resource_with_attribute",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().InsertString("app", "test")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					r.Attributes().InsertString(conventions.AttributeServiceName, "otlp_col")
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("service:otlp_col"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":                   "test",
					"status":                "debug",
					"otel.serverity_number": "5",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Transform(tt.args.lr, tt.args.res)

			gs, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
				return
			}
			ws, err := tt.want.MarshalJSON()
			if err != nil {
				t.Fatal(err)
				return
			}
			if !assert.JSONEq(t, string(ws), string(gs)) {
				t.Errorf("Transform() = %v, want %v", string(gs), string(ws))
			}
		})
	}
}

func Test_deriveStatus(t *testing.T) {
	type args struct {
		severity int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "trace",
			args: args{
				severity: 3,
			},
			want: LogLevelTrace,
		},
		{
			name: "debug",
			args: args{
				severity: 7,
			},
			want: LogLevelDebug,
		},
		{
			name: "warn",
			args: args{
				severity: 13,
			},
			want: LogLevelWarn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, deriveStatus(tt.args.severity), "deriveStatus(%v)", tt.args.severity)
		})
	}
}
