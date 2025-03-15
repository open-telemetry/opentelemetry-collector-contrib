// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"fmt"
	"testing"
	"time"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

var emptyLogs = func() plog.Logs {
	ld := plog.NewLogs()
	return ld
}

var logRecordWithMissingKind = func() plog.Logs {
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	lrs := r.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp")
	return ld
}

var logRecordWithUnknownKind = func() plog.Logs {
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	lrs := r.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=info message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp")
	return ld
}

var twoIdenticalLogRecordsWithDifferentServiceNameResourceAttribute = func() plog.Logs {
	ld := plog.NewLogs()
	r1 := ld.ResourceLogs().AppendEmpty()
	r1.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp")
	lr1 := r1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr1.Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp")
	r2 := ld.ResourceLogs().AppendEmpty()
	r2.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp-second")
	lr2 := r2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr2.Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp")

	return ld
}

var twoLogRecordsWithDifferentAppRelease = func() plog.Logs {
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	lrs := r.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp app_release=1.2.3")
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp app_release=4.5.6")

	return ld
}

var twoLogRecordsWithTheSameResource = func() plog.Logs {
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	r.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "testapp")
	lrs := r.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def app_name=testapp")
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"loading price list\" level=trace context_component=AppRoot context_page=Pricing traceID=abcd spanID=ghj app_name=testapp")

	return ld
}

var multipleLogRecordsWithTheSameResource = func() plog.Logs {
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	resAttrs := r.Resource().Attributes()
	resAttrs.PutStr(string(semconv.ServiceNameKey), "testapp")
	resAttrs.PutStr(string(semconv.ServiceVersionKey), "abcdefg")
	resAttrs.PutStr(string(semconv.ServiceNamespaceKey), "testnamespace")
	resAttrs.PutStr(string(semconv.DeploymentEnvironmentKey), "production")
	resAttrs.PutStr("app_bundle_id", "testBundleId")

	lrs := r.ScopeLogs().AppendEmpty().LogRecords()
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=log message=\"loading price list\" level=trace context_component=AppRoot context_page=Pricing traceID=abcd spanID=ghj sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=exception type=Error value=\"Cannot read property 'find' of undefined\" stacktrace=\"Error: Cannot read property 'find' of undefined\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\\n  at dispatchAction (http://fe:3002/static/js/vendors~main.chunk.js:268095:9)\\n  at scheduleUpdateOnFiber (http://fe:3002/static/js/vendors~main.chunk.js:273726:13)\\n  at flushSyncCallbackQueue (http://fe:3002/static/js/vendors~main.chunk.js:263362:7)\\n  at flushSyncCallbackQueueImpl (http://fe:3002/static/js/vendors~main.chunk.js:263374:13)\\n  at runWithPriority$1 (http://fe:3002/static/js/vendors~main.chunk.js:263325:14)\\n  at unstable_runWithPriority (http://fe:3002/static/js/vendors~main.chunk.js:291265:16)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:263379:30)\\n  at performSyncWorkOnRoot (http://fe:3002/static/js/vendors~main.chunk.js:274126:22)\\n  at renderRootSync (http://fe:3002/static/js/vendors~main.chunk.js:274509:11)\\n  at workLoopSync (http://fe:3002/static/js/vendors~main.chunk.js:274543:9)\\n  at performUnitOfWork (http://fe:3002/static/js/vendors~main.chunk.js:274606:16)\\n  at beginWork$1 (http://fe:3002/static/js/vendors~main.chunk.js:275746:18)\\n  at beginWork (http://fe:3002/static/js/vendors~main.chunk.js:270944:20)\\n  at updateFunctionComponent (http://fe:3002/static/js/vendors~main.chunk.js:269291:24)\\n  at renderWithHooks (http://fe:3002/static/js/vendors~main.chunk.js:266969:22)\\n  at ? (http://fe:3002/static/js/main.chunk.js:2600:74)\\n  at useGetBooksQuery (http://fe:3002/static/js/main.chunk.js:1299:65)\\n  at Module.useQuery (http://fe:3002/static/js/vendors~main.chunk.js:8495:85)\\n  at useBaseQuery (http://fe:3002/static/js/vendors~main.chunk.js:8656:83)\\n  at useDeepMemo (http://fe:3002/static/js/vendors~main.chunk.js:8696:14)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8657:55)\\n  at QueryData.execute (http://fe:3002/static/js/vendors~main.chunk.js:7883:47)\\n  at QueryData.getExecuteResult (http://fe:3002/static/js/vendors~main.chunk.js:7944:23)\\n  at QueryData._this.getQueryResult (http://fe:3002/static/js/vendors~main.chunk.js:7790:19)\\n  at new ApolloError (http://fe:3002/static/js/vendors~main.chunk.js:5164:24)\" traceID=abcd spanID=def context_ReactError=\"Annoying Error\" context_component=ReactErrorBoundary sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
	lrs.AppendEmpty().Body().SetStr("timestamp=2021-09-30T10:46:17.68Z kind=measurement type=\"page load\" context_hello=world ttfb=14.000000 ttfcp=22.120000 ttfp=20.120000 traceID=abcd spanID=def value_ttfb=14 value_ttfcp=22.12 value_ttfp=20.12 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")
	lrs.AppendEmpty().Body().SetStr("timestamp=2023-11-16T10:00:55.995Z kind=event event_name=faro.performanceEntry event_domain=browser event_data_connectEnd=3656 event_data_connectStart=337 event_data_decodedBodySize=0 event_data_domainLookupEnd=590 event_data_domainLookupStart=588 event_data_duration=3371 event_data_encodedBodySize=0 event_data_entryType=resource event_data_fetchStart=331 event_data_initiatorType=other event_data_name=https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/css/bootstrap.min.css.map event_data_nextHopProtocol=h2 event_data_redirectEnd=0 event_data_redirectStart=0 event_data_requestStart=3656 event_data_responseEnd=3702 event_data_responseStart=3690 event_data_secureConnectionStart=3638 event_data_serverTiming=[] event_data_startTime=331 event_data_transferSize=0 event_data_workerStart=0 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar")

	return ld
}

func TestTranslateFromLogs(t *testing.T) {
	testcases := []struct {
		name         string
		ld           plog.Logs
		wantPayloads []*faroTypes.Payload
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name: "Empty logs",
			ld:   emptyLogs(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Log body doesn't contain kind",
			ld:   logRecordWithMissingKind(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				return payloads
			}(),
			wantErr: assert.Error,
		},
		{
			name: "Log body contains unknown kind",
			ld:   logRecordWithUnknownKind(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				return payloads
			}(),
			wantErr: assert.Error,
		},
		{
			name: "Two identical log records with different service.name resource attribute should produce two faro payloads",
			ld:   twoIdenticalLogRecordsWithDifferentServiceNameResourceAttribute(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-meta-app-name-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "payload-meta-app-name-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Two log records with the same resource should produce one faro payload",
			ld:   twoLogRecordsWithTheSameResource(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-two-records-same-resource.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Two log records with different app_release in log body should produce two faro payloads",
			ld:   twoLogRecordsWithDifferentAppRelease(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-meta-app-release-1.json"))
				payloads = append(payloads, PayloadFromFile(t, "payload-meta-app-release-2.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "Multiple log records of different kinds with the same resource should produce one faro payload",
			ld:   multipleLogRecordsWithTheSameResource(),
			wantPayloads: func() []*faroTypes.Payload {
				payloads := make([]*faroTypes.Payload, 0)
				payloads = append(payloads, PayloadFromFile(t, "payload-multiple-records-same-resource.json"))
				return payloads
			}(),
			wantErr: assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			faroPayloads, err := TranslateFromLogs(context.TODO(), tt.ld)
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
		wantPayload *faroTypes.Payload
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
			wantPayload: nil,
			wantErr:     assert.Error,
		},
		{
			name: "Log record doesn't have body",
			lr: func() plog.LogRecord {
				record := plog.NewLogRecord()
				return record
			}(),
			rl:          pcommon.NewResource(),
			wantPayload: nil,
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
			wantPayload: &faroTypes.Payload{
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
