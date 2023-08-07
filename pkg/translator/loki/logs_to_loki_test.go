// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestLogsToLokiRequestWithGroupingByTenant(t *testing.T) {
	tests := []struct {
		name                 string
		logs                 plog.Logs
		expected             map[string]PushRequest
		defaultLabelsEnabled map[string]bool
	}{
		{
			name: "tenant from logs attributes",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr(hintTenant, "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "1")
				logRecord.Attributes().PutInt("http.status", 200)

				sl = rl.ScopeLogs().AppendEmpty()
				logRecord = sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr(hintTenant, "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "2")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]PushRequest{
				"1": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="1"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								}},
						},
					},
				},
				"2": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="2"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "tenant from resource attributes",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr(hintTenant, "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "11")

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutInt("http.status", 200)

				rl = logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr(hintTenant, "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "12")

				sl = rl.ScopeLogs().AppendEmpty()
				logRecord = sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]PushRequest{
				"11": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="11"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								}},
						},
					},
				},
				"12": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="12"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "tenant hint attribute is not found in resource and logs attributes",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr(hintTenant, "tenant.id")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]PushRequest{
				"": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								}},
						},
					},
				},
			},
		},
		{
			name: "use tenant resource attributes if both logs and resource attributes provided",
			logs: func() plog.Logs {
				logs := plog.NewLogs()

				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr(hintTenant, "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "21")

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr(hintTenant, "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "31")
				logRecord.Attributes().PutInt("http.status", 200)

				rl = logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr(hintTenant, "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "22")

				sl = rl.ScopeLogs().AppendEmpty()
				logRecord = sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr(hintTenant, "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "32")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]PushRequest{
				"21": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="21"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								}},
						},
					},
				},
				"22": {
					PushRequest: &push.PushRequest{
						Streams: []push.Stream{
							{
								Labels: `{exporter="OTLP", tenant_id="22"}`,
								Entries: []push.Entry{
									{
										Line: `{"attributes":{"http.status":200}}`,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := LogsToLokiRequests(tt.logs, tt.defaultLabelsEnabled)

			for tenant, request := range requests {
				want, ok := tt.expected[tenant]
				assert.Equal(t, ok, true)

				streams := request.Streams
				for s := 0; s < len(streams); s++ {
					gotStream := request.Streams[s]
					wantStream := want.Streams[s]

					assert.Equal(t, wantStream.Labels, gotStream.Labels)
					for e := 0; e < len(gotStream.Entries); e++ {
						assert.Equal(t, wantStream.Entries[e].Line, gotStream.Entries[e].Line)
					}
				}
			}
		})
	}
}

func TestLogsToLokiRequestWithoutTenant(t *testing.T) {
	testCases := []struct {
		desc                 string
		hints                map[string]interface{}
		attrs                map[string]interface{}
		res                  map[string]interface{}
		severity             plog.SeverityNumber
		levelAttribute       string
		expectedLabel        string
		expectedLines        []string
		defaultLebelsEnabled map[string]bool
	}{
		{
			desc: "with attribute to label and regular attribute",
			attrs: map[string]interface{}{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
			},
			expectedLabel: `{exporter="OTLP", host_name="guarana"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000","attributes":{"http.status":200}}`,
				`{"traceid":"02000000000000000000000000000000","attributes":{"http.status":200}}`,
				`{"traceid":"03000000000000000000000000000000","attributes":{"http.status":200}}`,
			},
		},
		{
			desc: "with resource to label and regular resource",
			res: map[string]interface{}{
				"host.name": "guarana",
				"region.az": "eu-west-1a",
			},
			hints: map[string]interface{}{
				hintResources: "host.name",
			},
			expectedLabel: `{exporter="OTLP", host_name="guarana"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
				`{"traceid":"02000000000000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
				`{"traceid":"03000000000000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
			},
		},
		{
			desc: "with logfmt format",
			attrs: map[string]interface{}{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
				hintFormat:     formatLogfmt,
			},
			expectedLabel: `{exporter="OTLP", host_name="guarana"}`,
			expectedLines: []string{
				`traceID=01000000000000000000000000000000 attribute_http.status=200`,
				`traceID=02000000000000000000000000000000 attribute_http.status=200`,
				`traceID=03000000000000000000000000000000 attribute_http.status=200`,
			},
		},
		{
			desc:          "with severity to label",
			severity:      plog.SeverityNumberDebug4,
			expectedLabel: `{exporter="OTLP", level="DEBUG4"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc:     "with severity to label, but default_labels_enable disables level label",
			severity: plog.SeverityNumberDebug4,
			defaultLebelsEnabled: map[string]bool{
				levelLabel: false,
			},
			expectedLabel: `{exporter="OTLP"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc:           "with severity, already existing level",
			severity:       plog.SeverityNumberDebug4,
			levelAttribute: "dummy",
			expectedLabel:  `{exporter="OTLP", level="dummy"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc:           "with severity, already existing level, but default_labels_enable disables level label",
			severity:       plog.SeverityNumberDebug4,
			levelAttribute: "dummy",
			defaultLebelsEnabled: map[string]bool{
				levelLabel: false,
			},
			expectedLabel: `{exporter="OTLP"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000","attributes":{"level":"dummy"}}`,
				`{"traceid":"02000000000000000000000000000000","attributes":{"level":"dummy"}}`,
				`{"traceid":"03000000000000000000000000000000","attributes":{"level":"dummy"}}`,
			},
		},
		{
			desc: "with severity, already existing level and hint attribute",
			attrs: map[string]interface{}{
				"host.name": "guarana",
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
			},
			severity:       plog.SeverityNumberDebug4,
			levelAttribute: "dummy",
			expectedLabel:  `{exporter="OTLP", host_name="guarana", level="dummy"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc: "with existing level and hint attributes contain level",
			attrs: map[string]interface{}{
				"host.name": "guarana",
			},
			hints: map[string]interface{}{
				hintAttributes: "level, host.name",
			},
			levelAttribute: "dummy",
			expectedLabel:  `{exporter="OTLP", host_name="guarana", level="dummy"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc: "with hint attributes and resource attributes as string",
			attrs: map[string]interface{}{
				"host.name": "guarana",
				"host.ip":   "127.0.0.1",
			},
			res: map[string]interface{}{
				"service.name":      "my-service",
				"service.namespace": "my-namespace",
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name, host.ip",
				hintResources:  "service.name, service.namespace",
			},
			severity:      plog.SeverityNumberDebug4,
			expectedLabel: `{exporter="OTLP", host_ip="127.0.0.1", host_name="guarana", job="my-namespace/my-service", level="DEBUG4", service_name="my-service", service_namespace="my-namespace"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
		{
			desc: "with hint attributes as slice",
			attrs: map[string]interface{}{
				"host.name": "guarana",
				"host.ip":   "127.0.0.1",
			},
			res: map[string]interface{}{
				"service.name":      "my-service",
				"service.namespace": "my-namespace",
			},
			hints: map[string]interface{}{
				hintAttributes: []any{"host.name", "host.ip"},
				hintResources:  []any{"service.name", "service.namespace"},
			},
			severity:      plog.SeverityNumberDebug4,
			expectedLabel: `{exporter="OTLP", host_ip="127.0.0.1", host_name="guarana", job="my-namespace/my-service", level="DEBUG4", service_name="my-service", service_namespace="my-namespace"}`,
			expectedLines: []string{
				`{"traceid":"01000000000000000000000000000000"}`,
				`{"traceid":"02000000000000000000000000000000"}`,
				`{"traceid":"03000000000000000000000000000000"}`,
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			// prepare
			ld := plog.NewLogs()
			ld.ResourceLogs().AppendEmpty()
			for i := 0; i < 3; i++ {
				ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
				ld.ResourceLogs().At(0).ScopeLogs().At(i).LogRecords().AppendEmpty()
				ld.ResourceLogs().At(0).ScopeLogs().At(i).LogRecords().At(0).SetTraceID([16]byte{byte(i + 1)})
				ld.ResourceLogs().At(0).ScopeLogs().At(i).LogRecords().At(0).SetSeverityNumber(tt.severity)
			}

			if len(tt.res) > 0 {
				res := tt.res
				if val, ok := tt.hints[hintResources]; ok {
					res[hintResources] = val
				}
				assert.NoError(t, ld.ResourceLogs().At(0).Resource().Attributes().FromRaw(res))
			}

			rlogs := ld.ResourceLogs()
			for i := 0; i < rlogs.Len(); i++ {
				slogs := rlogs.At(i).ScopeLogs()
				for j := 0; j < slogs.Len(); j++ {
					logs := slogs.At(j).LogRecords()
					for k := 0; k < logs.Len(); k++ {
						log := logs.At(k)
						attrs := map[string]interface{}{}
						if len(tt.attrs) > 0 {
							attrs = tt.attrs
						}

						if len(tt.levelAttribute) > 0 {
							attrs[levelAttributeName] = tt.levelAttribute
						}
						if val, ok := tt.hints[hintAttributes]; ok {
							attrs[hintAttributes] = val
						}
						if val, ok := tt.hints[hintFormat]; ok {
							attrs[hintFormat] = val
						}
						if len(attrs) > 0 {
							assert.NoError(t, log.Attributes().FromRaw(attrs))
						}
					}
				}
			}

			// test
			requests := LogsToLokiRequests(ld, tt.defaultLebelsEnabled)
			assert.Len(t, requests, 1)
			request := requests[""]

			// verify
			assert.Empty(t, request.Report.Errors)
			assert.Equal(t, 0, request.Report.NumDropped)
			assert.Equal(t, ld.LogRecordCount(), request.Report.NumSubmitted)
			assert.Len(t, request.Streams, 1)
			assert.Equal(t, tt.expectedLabel, request.Streams[0].Labels)

			entries := request.Streams[0].Entries
			for i := 0; i < len(entries); i++ {
				assert.Equal(t, tt.expectedLines[i], entries[i].Line)
			}
		})
	}
}

func TestLogToLokiEntry(t *testing.T) {
	testCases := []struct {
		name                 string
		timestamp            time.Time
		severity             plog.SeverityNumber
		levelAttribute       string
		res                  map[string]interface{}
		attrs                map[string]interface{}
		hints                map[string]interface{}
		instrumentationScope *instrumentationScope
		expected             *PushEntry
		err                  error
		defaultLabelsEnabled map[string]bool
	}{
		{
			name:      "with attribute to label and regular attribute",
			timestamp: time.Unix(0, 1677592916000000000),
			attrs: map[string]interface{}{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
			},
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      `{"attributes":{"http.status":200}}`,
				},
				Labels: model.LabelSet{
					"exporter":  "OTLP",
					"host.name": "guarana",
				},
			},
			err: nil,
		},
		{
			name:      "with resource to label and regular resource",
			timestamp: time.Unix(0, 1677592916000000000),
			res: map[string]interface{}{
				"host.name": "guarana",
				"region.az": "eu-west-1a",
			},
			hints: map[string]interface{}{
				hintResources: "host.name",
			},
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      `{"resources":{"region.az":"eu-west-1a"}}`,
				},
				Labels: model.LabelSet{
					"exporter":  "OTLP",
					"host.name": "guarana",
				},
			},
		},
		{
			name:      "with logfmt format",
			timestamp: time.Unix(0, 1677592916000000000),
			attrs: map[string]interface{}{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
				hintFormat:     formatLogfmt,
			},
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      `attribute_http.status=200`,
				},
				Labels: model.LabelSet{
					"exporter":  "OTLP",
					"host.name": "guarana",
				},
			},
		},
		{
			name:      "with severity to label",
			timestamp: time.Unix(0, 1677592916000000000),
			severity:  plog.SeverityNumberDebug4,
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      "{}",
				},
				Labels: model.LabelSet{
					"exporter": "OTLP",
					"level":    "DEBUG4",
				},
			},
		},
		{
			name:           "with severity, already existing level",
			timestamp:      time.Unix(0, 1677592916000000000),
			severity:       plog.SeverityNumberDebug4,
			levelAttribute: "dummy",
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      "{}",
				},
				Labels: model.LabelSet{
					"exporter": "OTLP",
					"level":    "dummy",
				},
			},
		},
		{
			name:      "with instrumentation scope",
			timestamp: time.Unix(0, 1677592916000000000),
			instrumentationScope: &instrumentationScope{
				Name:    "otlp",
				Version: "v1",
			},
			expected: &PushEntry{
				Entry: &push.Entry{
					Timestamp: time.Unix(0, 1677592916000000000),
					Line:      `{"instrumentation_scope":{"name":"otlp","version":"v1"}}`,
				},
				Labels: model.LabelSet{
					"exporter": "OTLP",
				},
			},
		},
		{
			name:      "with unknown format hint",
			timestamp: time.Unix(0, 1677592916000000000),
			hints: map[string]interface{}{
				hintFormat: "my-format",
			},
			expected: nil,
			err:      fmt.Errorf("invalid format %s. Expected one of: %s, %s, %s", "my-format", formatJSON, formatLogfmt, formatRaw),
		},
	}

	for _, tt := range testCases {
		if tt.name == "with unknown format hint" {
			t.Run(tt.name, func(t *testing.T) {
				t.Skipf("skipping test '%v'. see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/20240 for details.", tt.name)
			})
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			lr.SetTimestamp(pcommon.NewTimestampFromTime(tt.timestamp))

			err := lr.Attributes().FromRaw(tt.attrs)
			require.NoError(t, err)
			for k, v := range tt.hints {
				lr.Attributes().PutStr(k, fmt.Sprintf("%v", v))
			}

			scope := pcommon.NewInstrumentationScope()
			if tt.instrumentationScope != nil {
				scope.SetName(tt.instrumentationScope.Name)
				scope.SetVersion(tt.instrumentationScope.Version)
			}

			resource := pcommon.NewResource()
			err = resource.Attributes().FromRaw(tt.res)
			require.NoError(t, err)
			for k, v := range tt.hints {
				resource.Attributes().PutStr(k, fmt.Sprintf("%v", v))
			}
			lr.SetSeverityNumber(tt.severity)
			if len(tt.levelAttribute) > 0 {
				lr.Attributes().PutStr(levelAttributeName, tt.levelAttribute)
			}

			log, err := LogToLokiEntry(lr, resource, scope, tt.defaultLabelsEnabled)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.expected, log)
		})
	}
}

func TestGetTenantFromTenantHint(t *testing.T) {
	testCases := []struct {
		name     string
		attrs    map[string]interface{}
		res      map[string]interface{}
		expected string
	}{
		{
			name: "tenant in attributes",
			attrs: map[string]interface{}{
				hintTenant:  "tenant.id",
				"tenant.id": "1",
			},
			expected: "1",
		},
		{
			name: "tenant in resources",
			res: map[string]interface{}{
				hintTenant:  "tenant.id",
				"tenant.id": "1",
			},
			expected: "1",
		},
		{
			name: "if tenant set in resources and attributes, the one in resource should win",
			res: map[string]interface{}{
				hintTenant:  "tenant.id",
				"tenant.id": "1",
			},
			attrs: map[string]interface{}{
				hintTenant:  "tenant.id",
				"tenant.id": "2",
			},
			expected: "1",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			err := lr.Attributes().FromRaw(tt.attrs)
			require.NoError(t, err)

			resource := pcommon.NewResource()
			err = resource.Attributes().FromRaw(tt.res)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, GetTenantFromTenantHint(lr.Attributes(), resource.Attributes()))
		})
	}
}
