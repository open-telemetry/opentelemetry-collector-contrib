// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

func TestTranslateFromLogs(t *testing.T) {
	testcases := []struct {
		name         string
		plogsFile    string
		wantPayloads []faroTypes.Payload
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:         "Empty logs",
			plogsFile:    filepath.Join("testdata", "empty-payload", "plogs.yaml"),
			wantPayloads: []faroTypes.Payload{},
			wantErr:      assert.NoError,
		},
		{
			name:         "Log body doesn't contain kind",
			plogsFile:    filepath.Join("testdata", "plogs-record-missing-kind.yaml"),
			wantPayloads: []faroTypes.Payload{},
			wantErr:      assert.Error,
		},
		{
			name:         "Log body contains unknown kind",
			plogsFile:    filepath.Join("testdata", "plogs-record-unknown-kind.yaml"),
			wantPayloads: []faroTypes.Payload{},
			wantErr:      assert.Error,
		},
		{
			name:      "Two identical log records with different service.name resource attribute should produce two faro payloads",
			plogsFile: filepath.Join("testdata", "two-identical-log-records-different-service-name-resource-attribute", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "two-identical-log-records-different-service-name-resource-attribute/payload-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "two-identical-log-records-different-service-name-resource-attribute/payload-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Two log records with the same resource should produce one faro payload",
			plogsFile: filepath.Join("testdata", "two-log-records-same-resource", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "two-log-records-same-resource/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Two log records with different app_release in log body should produce two faro payloads",
			plogsFile: filepath.Join("testdata", "two-log-records-different-app-release", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "two-log-records-different-app-release/payload-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "two-log-records-different-app-release/payload-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Multiple log records of different kinds with the same resource should produce one faro payload",
			plogsFile: filepath.Join("testdata", "multiple-log-records-same-resource", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "multiple-log-records-same-resource/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Log record with action",
			plogsFile: filepath.Join("testdata", "actions-payload", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "actions-payload/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Log record with browser brands as slice",
			plogsFile: filepath.Join("testdata", "browser-brand-slice-payload", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "browser-brand-slice-payload/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name:      "Log record with browser brands as string",
			plogsFile: filepath.Join("testdata", "browser-brand-string-payload", "plogs.yaml"),
			wantPayloads: func() []faroTypes.Payload {
				payloads := make([]faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "browser-brand-string-payload/payload.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			plogs, err := golden.ReadLogs(tt.plogsFile)
			require.NoError(t, err)
			faroPayloads, err := TranslateFromLogs(context.TODO(), plogs)
			tt.wantErr(t, err)
			assert.ElementsMatch(t, tt.wantPayloads, faroPayloads)
		})
	}
}

func Test_extractBrowserBrandsFromKeyVal(t *testing.T) {
	tests := []struct {
		name       string
		kv         map[string]string
		wantErr    assert.ErrorAssertionFunc
		wantBrands faroTypes.Browser_Brands
	}{
		{
			name: "brands as string",
			kv: map[string]string{
				"browser_brands": "Chromium;Google Inc.;",
			},
			wantErr: assert.NoError,
			wantBrands: func(t *testing.T) faroTypes.Browser_Brands {
				var brands faroTypes.Browser_Brands
				err := brands.FromBrandsString("Chromium;Google Inc.;")
				require.NoError(t, err)
				return brands
			}(t),
		},
		{
			name: "brands as array",
			kv: map[string]string{
				"browser_brand_0_brand":   "brand1",
				"browser_brand_0_version": "0.1.0",
				"browser_brand_1_brand":   "brand2",
				"browser_brand_1_version": "0.2.0",
			},
			wantErr: assert.NoError,
			wantBrands: func(t *testing.T) faroTypes.Browser_Brands {
				var brands faroTypes.Browser_Brands
				err := brands.FromBrandsArray(faroTypes.BrandsArray{
					{
						Brand:   "brand1",
						Version: "0.1.0",
					},
					{
						Brand:   "brand2",
						Version: "0.2.0",
					},
				})
				require.NoError(t, err)
				return brands
			}(t),
		},
		{
			name:    "brands are missing",
			kv:      map[string]string{},
			wantErr: assert.NoError,
			wantBrands: func(_ *testing.T) faroTypes.Browser_Brands {
				var brands faroTypes.Browser_Brands
				return brands
			}(t),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brands, err := extractBrowserBrandsFromKeyVal(tt.kv)
			tt.wantErr(t, err, fmt.Sprintf("extractBrowserBrandsFromKeyVal(%v)", tt.kv))
			assert.Equal(t, tt.wantBrands, brands)
		})
	}
}

func Test_extractK6FromKeyVal(t *testing.T) {
	testcases := []struct {
		name    string
		kv      map[string]string
		want    faroTypes.K6
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "k6_isK6Browser is missing in kv",
			kv:      map[string]string{},
			want:    faroTypes.K6{},
			wantErr: assert.NoError,
		},
		{
			name: "k6_isK6Browser can't be parsed as boolean",
			kv: map[string]string{
				"k6_isK6Browser": "foo",
			},
			want:    faroTypes.K6{},
			wantErr: assert.Error,
		},
		{
			name: "k6_isK6Browser can be parsed as true",
			kv: map[string]string{
				"k6_isK6Browser": "true",
			},
			want: faroTypes.K6{
				IsK6Browser: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "k6_isK6Browser can be parsed as false",
			kv: map[string]string{
				"k6_isK6Browser": "0",
			},
			want: faroTypes.K6{
				IsK6Browser: false,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractK6FromKeyVal(tt.kv)
			if !tt.wantErr(t, err, fmt.Sprintf("extractK6FromKeyVal(%v)", tt.kv)) {
				return
			}
			assert.Equalf(t, tt.want, got, "extractK6FromKeyVal(%v)", tt.kv)
		})
	}
}

func Test_parseFrameFromString(t *testing.T) {
	testcases := []struct {
		name     string
		frameStr string
		frame    *faroTypes.Frame
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "Frame string is empty",
			frameStr: "",
			frame:    nil,
			wantErr:  assert.NoError,
		},
		{
			name:     "All the frame fields are present in the frame string",
			frameStr: "? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)",
			frame: &faroTypes.Frame{
				Colno:    42,
				Lineno:   8639,
				Function: "?",
				Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
			},
			wantErr: assert.NoError,
		},
		{
			name:     "Function contains spaces",
			frameStr: "new ApolloError (http://fe:3002/static/js/vendors~main.chunk.js:5164:24)",
			frame: &faroTypes.Frame{
				Colno:    24,
				Lineno:   5164,
				Function: "new ApolloError",
				Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
			},
			wantErr: assert.NoError,
		},
		{
			name:     "Module name is present in the frame string",
			frameStr: "? (module_name|http://fe:3002/static/js/vendors~main.chunk.js:8639:42)",
			frame: &faroTypes.Frame{
				Colno:    42,
				Lineno:   8639,
				Function: "?",
				Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
				Module:   "module_name",
			},
			wantErr: assert.NoError,
		},
		{
			name:     "Function name is empty",
			frameStr: " (module_name|http://fe:3002/static/js/vendors~main.chunk.js:8639:42)",
			frame: &faroTypes.Frame{
				Colno:    42,
				Lineno:   8639,
				Function: "",
				Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
				Module:   "module_name",
			},
			wantErr: assert.NoError,
		},
		{
			name:     "Filename is empty",
			frameStr: " (module_name|:8639:42)",
			frame: &faroTypes.Frame{
				Colno:    42,
				Lineno:   8639,
				Function: "",
				Filename: "",
				Module:   "module_name",
			},
			wantErr: assert.NoError,
		},
		{
			name:     "Lineno, colno are empty",
			frameStr: " (module_name|::)",
			frame: &faroTypes.Frame{
				Colno:    0,
				Lineno:   0,
				Function: "",
				Filename: "",
				Module:   "module_name",
			},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := parseFrameFromString(tt.frameStr)
			tt.wantErr(t, err, fmt.Sprintf("parseFrameFromString(%s)", tt.frameStr))
			assert.Equal(t, tt.frame, frame)
		})
	}
}

func Test_translateLogToFaroPayload(t *testing.T) {
	testcases := []struct {
		name        string
		lr          plog.LogRecord
		rl          pcommon.Resource
		wantPayload faroTypes.Payload
		wantErr     assert.ErrorAssertionFunc
	}{
		{
			name: "Malformed log record body",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("foo bar")
				return record
			}(),
			rl:          pcommon.NewResource(),
			wantPayload: faroTypes.Payload{},
			wantErr:     assert.Error,
		},
		{
			name: "Log record doesn't have body",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				return record
			}(),
			rl:          pcommon.NewResource(),
			wantPayload: faroTypes.Payload{},
			wantErr:     assert.Error,
		},
		{
			name: "Log record body has kind=log only",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("kind=log")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Logs: []faroTypes.Log{
					{},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=log and other fields",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Logs: []faroTypes.Log{
					{
						Context: map[string]string{
							"component": "AppRoot",
							"page":      "Pricing",
						},
						Timestamp: time.Date(2021, 9, 30, 10, 46, 17, 680000000, time.UTC),
						Message:   "opened pricing page",
						LogLevel:  faroTypes.LogLevelInfo,
						Trace: faroTypes.TraceContext{
							TraceID: "abcd",
							SpanID:  "def",
						},
					},
				},
				Meta: getTestMeta(),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=event",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("kind=event")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Events: []faroTypes.Event{
					{},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=event and other fields",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("timestamp=2023-11-16T10:00:55.995Z kind=event event_name=faro.performanceEntry event_domain=browser event_data_connectEnd=3656 event_data_connectStart=337 event_data_decodedBodySize=0 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Events: []faroTypes.Event{
					{
						Name:      "faro.performanceEntry",
						Domain:    "browser",
						Timestamp: time.Date(2023, 11, 16, 10, 0, 55, 995000000, time.UTC),
						Attributes: map[string]string{
							"connectEnd":      "3656",
							"connectStart":    "337",
							"decodedBodySize": "0",
						},
					},
				},
				Meta: getTestMeta(),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=measurement",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("kind=measurement")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Measurements: []faroTypes.Measurement{
					{},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=measurement and other fields",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=measurement type=\"page load\" context_hello=world ttfb=14.000000 ttfcp=22.120000 ttfp=20.120000 traceID=abcd spanID=def value_ttfb=14 value_ttfcp=22.12 value_ttfp=20.12 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Measurements: []faroTypes.Measurement{
					{
						Timestamp: time.Date(2021, 9, 30, 10, 46, 17, 680000000, time.UTC),
						Context: map[string]string{
							"hello": "world",
						},
						Type: "page load",
						Trace: faroTypes.TraceContext{
							TraceID: "abcd",
							SpanID:  "def",
						},
						Values: map[string]float64{
							"ttfb":  14,
							"ttfcp": 22.12,
							"ttfp":  20.12,
						},
					},
				},
				Meta: getTestMeta(),
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=exception",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("kind=exception")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Exceptions: []faroTypes.Exception{
					{},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "Log record body has kind=exception and other fields",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				record.Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=exception type=Error value=\"Cannot read property 'find' of undefined\" stacktrace=\"Error: Cannot read property 'find' of undefined\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\" traceID=abcd spanID=def context_ReactError=\"Annoying Error\" context_component=ReactErrorBoundary sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
				return record
			}(),
			rl: pcommon.NewResource(),
			wantPayload: faroTypes.Payload{
				Exceptions: []faroTypes.Exception{
					{
						Timestamp: time.Date(2021, 9, 30, 10, 46, 17, 680000000, time.UTC),
						Context: map[string]string{
							"ReactError": "Annoying Error",
							"component":  "ReactErrorBoundary",
						},
						Type: "Error",
						Trace: faroTypes.TraceContext{
							TraceID: "abcd",
							SpanID:  "def",
						},
						Value: "Cannot read property 'find' of undefined",
						Stacktrace: &faroTypes.Stacktrace{
							Frames: []faroTypes.Frame{
								{
									Colno:    42,
									Lineno:   8639,
									Filename: "http://fe:3002/static/js/vendors~main.chunk.js",
									Function: "?",
								},
							},
						},
					},
				},
				Meta: getTestMeta(),
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translateLogToFaroPayload(tt.lr, tt.rl)
			if !tt.wantErr(t, err, fmt.Sprintf("translateLogToFaroPayload(%v, %v)", tt.lr, tt.rl)) {
				return
			}
			assert.Equalf(t, tt.wantPayload, got, "translateLogToFaroPayload(%v, %v)", tt.lr, tt.rl)
		})
	}
}

func Test_parseIntegrationsFromString(t *testing.T) {
	tests := []struct {
		name               string
		integrationsString string
		want               []faroTypes.SDKIntegration
	}{
		{
			name: "Integrations string is empty",
			want: []faroTypes.SDKIntegration{},
		},
		{
			name:               "Integrations string contain several integrations",
			integrationsString: "foo:1.2,example:3.4.5",
			want: []faroTypes.SDKIntegration{
				{
					Name:    "foo",
					Version: "1.2",
				},
				{
					Name:    "example",
					Version: "3.4.5",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIntegrationsFromString(tt.integrationsString)
			assert.Equalf(t, tt.want, got, "parseIntegrationsFromString(%v)", tt.integrationsString)
		})
	}
}

func Test_mergePayloads(t *testing.T) {
	tests := []struct {
		name            string
		targetPayload   *faroTypes.Payload
		sourcePayload   faroTypes.Payload
		expectedPayload *faroTypes.Payload
	}{
		{
			name:          "target.Traces is nil",
			targetPayload: &faroTypes.Payload{},
			sourcePayload: faroTypes.Payload{
				Logs: []faroTypes.Log{
					{
						Message: "test",
					},
				},
				Events: []faroTypes.Event{
					{
						Name: "test",
					},
				},
				Measurements: []faroTypes.Measurement{
					{
						Type: "test",
					},
				},
				Exceptions: []faroTypes.Exception{
					{
						Value: "test",
					},
				},
				Traces: func() *faroTypes.Traces {
					traces := ptrace.NewTraces()
					traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("test")
					return &faroTypes.Traces{
						Traces: traces,
					}
				}(),
			},
			expectedPayload: &faroTypes.Payload{
				Logs: []faroTypes.Log{
					{
						Message: "test",
					},
				},
				Events: []faroTypes.Event{
					{
						Name: "test",
					},
				},
				Measurements: []faroTypes.Measurement{
					{
						Type: "test",
					},
				},
				Exceptions: []faroTypes.Exception{
					{
						Value: "test",
					},
				},
				Traces: func() *faroTypes.Traces {
					traces := ptrace.NewTraces()
					traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("test")
					return &faroTypes.Traces{
						Traces: traces,
					}
				}(),
			},
		},
		{
			name: "target payload doesn't have Logs, source payload has Logs",
			targetPayload: &faroTypes.Payload{
				Events: []faroTypes.Event{
					{
						Name: "test",
					},
				},
			},
			sourcePayload: faroTypes.Payload{
				Logs: []faroTypes.Log{
					{
						Message: "test2",
					},
				},
			},
			expectedPayload: &faroTypes.Payload{
				Logs: []faroTypes.Log{
					{
						Message: "test2",
					},
				},
				Events: []faroTypes.Event{
					{
						Name: "test",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergePayloads(tt.targetPayload, tt.sourcePayload)

			require.Equalf(t, tt.expectedPayload, tt.targetPayload, "mergePayloads()")
		})
	}
}

func getTestMeta() faroTypes.Meta {
	return faroTypes.Meta{
		App: faroTypes.App{
			Name:        "testapp",
			Namespace:   "testnamespace",
			Release:     "0.8.2",
			Version:     "abcdefg",
			Environment: "production",
		},
		SDK: faroTypes.SDK{
			Name:    "grafana-frontend-agent",
			Version: "1.3.5",
		},
		User: faroTypes.User{
			ID:       "123",
			Username: "testuser",
			Attributes: map[string]string{
				"foo": "bar",
			},
			Email: "geralt@kaermorhen.org",
		},
		Browser: faroTypes.Browser{
			Name:    "chrome",
			Version: "88.12.1",
			OS:      "linux",
			Mobile:  false,
			Brands:  faroTypes.Browser_Brands{},
		},
		View: faroTypes.View{
			Name: "foobar",
		},
		Session: faroTypes.Session{
			ID: "abcd",
			Attributes: map[string]string{
				"time_elapsed": "100s",
			},
		},
		Page: faroTypes.Page{
			URL: "https://example.com/page",
		},
	}
}
