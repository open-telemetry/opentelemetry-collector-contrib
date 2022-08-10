// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:errcheck
package syslogexporter

import (
	"context"
	"github.com/influxdata/go-syslog/v3/rfc5424"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
)

type mockSyslogClient struct {
	messages []string
}

func (m *mockSyslogClient) SendLog(s string) error {
	m.messages = append(m.messages, s)
	return nil
}

func (m *mockSyslogClient) Close() error {
	return nil
}

func createLogData(numberOfLogs int, resAttrRaw map[string]any, attrRaw map[string]any) plog.Logs {
	logs := plog.NewLogs()
	lrs := logs.ResourceLogs().AppendEmpty()

	attr := pcommon.NewMapFromRaw(attrRaw)
	resAttr := pcommon.NewMapFromRaw(resAttrRaw)

	resAttr.Range(func(k string, v pcommon.Value) bool {
		lrs.Resource().Attributes().Insert(k, v)
		return true
	})

	sl := lrs.ScopeLogs().AppendEmpty()
	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStringVal("mylog")
		attr.Range(func(k string, v pcommon.Value) bool {
			logRecord.Attributes().Insert(k, v)
			return true
		})
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func defaultConfig() Config {
	genericConfig := createDefaultConfig().(*Config)
	genericConfig.Endpoint = "noop"
	genericConfig.Validate()
	return *genericConfig
}

func TestExporter_PushEvent(t *testing.T) {

	tests := []struct {
		name         string
		validateFunc func(t *testing.T, m []rfc5424.SyslogMessage)
		config       func() Config
		genLogsFunc  func() plog.Logs
		errFunc      func(err error)
	}{
		{
			name:   "No SD when no attributes added",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					assert.Nil(t, message.StructuredData)
				}
			},
		},
		{
			name:   "SD added when attributes exist",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{
					"resource.attr": "foo",
				}, map[string]any{
					"record.attr": "baz",
				})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					commonId := defaultConfig().SdConfig.CommonSdId
					assert.Equal(t, "foo", sd[commonId]["resource.attr"])
					assert.Equal(t, "baz", sd[commonId]["record.attr"])
				}
			},
		},
		{
			name:   "Standard fields added from log properties",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					assert.Equal(t, "mylog\n", *message.Message)
					assert.Equal(t, time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), *message.Timestamp)
					assert.Equal(t, uint8(0), *message.Priority)
				}
			},
		},
		{
			name:   "Standard fields added when attributes present",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{
					"service.name":  "foo",
					"host.name":     "bar",
					"syslog.procid": "1",
					"syslog.msgid":  "2",
				}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					assert.Equal(t, "foo", *message.Appname)
					assert.Equal(t, "bar", *message.Hostname)
					assert.Equal(t, "1", *message.ProcID)
					assert.Equal(t, "2", *message.MsgID)
				}
			},
		},
		{
			name: "Common SDID renamed when config option set",
			config: func() Config {
				config := defaultConfig()
				config.SdConfig = SdConfig{CommonSdId: "custom"}
				return config
			},
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{
					"foo": "bar",
				}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					assert.Equal(t, "bar", sd["custom"]["foo"])
				}
			},
		},
		{
			name:   "Trace parameters added when available in log record",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				logData := createLogData(1, map[string]any{}, map[string]any{})
				traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
				spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 2}
				logRecord := logData.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))

				return logData
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					sdid := defaultConfig().SdConfig.TraceSdId
					assert.Equal(t, "00000000000000000000000000000001", sd[sdid]["trace_id"])
					assert.Equal(t, "0000000000000002", sd[sdid]["span_id"])
				}
			},
		},
		{
			name: "Trace SDID renamed when config option set",
			config: func() Config {
				config := defaultConfig()
				config.SdConfig = SdConfig{TraceSdId: "custom_trace"}
				return config
			},
			genLogsFunc: func() plog.Logs {
				logData := createLogData(1, map[string]any{}, map[string]any{})
				traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
				spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 2}
				logRecord := logData.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))
				return logData
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					assert.Equal(t, "00000000000000000000000000000001", sd["custom_trace"]["trace_id"])
					assert.Equal(t, "0000000000000002", sd["custom_trace"]["span_id"])
				}
			},
		},
		{
			name: "Common SdId changed if config option set",
			config: func() Config {
				config := defaultConfig()
				config.SdConfig = SdConfig{CommonSdId: "custom_sdid"}
				return config
			},
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{
					"resource.attr": "attr.val",
				}, map[string]any{
					"record.attr": "record.attr.val",
				})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					assert.Equal(t, "attr.val", sd["custom_sdid"]["resource.attr"])
					assert.Equal(t, "record.attr.val", sd["custom_sdid"]["record.attr"])
				}
			},
		},
		{
			name: "Static parameters added",
			config: func() Config {
				config := defaultConfig()
				config.SdConfig = SdConfig{StaticSd: map[string]map[string]string{"static_sdid1": {"sd1_key": "sd1_val"},
					"static_sd2": {"sd2_key": "sd2_val"}}}
				return config
			},
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					assert.Equal(t, "sd1_val", sd["static_sdid1"]["sd1_key"])
					assert.Equal(t, "sd2_val", sd["static_sd2"]["sd2_key"])
				}
			},
		},
		{
			name: "Parameter remapped when config set",
			config: func() Config {
				config := defaultConfig()
				config.SdConfig = SdConfig{CustomMapping: map[string]string{"attr1": "newsd", "attr2": "newsd2"}}
				return config
			},
			genLogsFunc: func() plog.Logs {
				return createLogData(1, map[string]any{"attr1": "value1"}, map[string]any{"attr2": "value2", "attr3": "value3"})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				for _, message := range m {
					sd := *message.StructuredData
					assert.Equal(t, "value1", sd["newsd"]["attr1"])
					assert.Equal(t, "value2", sd["newsd2"]["attr2"])
					assert.Equal(t, "value3", sd[defaultConfig().SdConfig.CommonSdId]["attr3"])
				}
			},
		},
		{
			name:   "Output number exact as input",
			config: defaultConfig,
			genLogsFunc: func() plog.Logs {
				return createLogData(10, map[string]any{}, map[string]any{})
			},
			validateFunc: func(t *testing.T, m []rfc5424.SyslogMessage) {
				assert.Len(t, m, 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockClient := mockSyslogClient{}
			exporter, err := newExporter(zaptest.NewLogger(t), tt.config(), &mockClient)
			assert.NoError(t, err)
			require.NotNil(t, exporter)
			//err := exporter.start(context.Background(), componenttest.NewNopHost())
			//require.NoError(t, err)

			err = exporter.ConsumeLogs(context.Background(), tt.genLogsFunc())

			if tt.errFunc != nil {
				tt.errFunc(err)
				return
			}

			assert.NoError(t, err)

			messages := mockClient.messages

			var sysLogMessages []rfc5424.SyslogMessage
			parser := rfc5424.NewParser()
			for _, message := range messages {
				parsed, err := parser.Parse([]byte(message))
				assert.NoError(t, err)
				sysLogMessages = append(sysLogMessages, *parsed.(*rfc5424.SyslogMessage))
			}
			tt.validateFunc(t, sysLogMessages)
		})
	}

	//func newTestExporter(t *testing.T, url string, fns ...func(*Config)) *syslogExporter {
	//
	//}
	//
	//func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	//
	//}
	//
	//func mustSend(t *testing.T, exporter *syslogExporter, contents string) {
	//
	//}
}
