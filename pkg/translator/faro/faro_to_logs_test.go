// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestTranslateToLogs(t *testing.T) {
	testcases := []struct {
		name         string
		faroPayload  faroTypes.Payload
		expectedLogs *plog.Logs
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:         "Empty payload",
			faroPayload:  faroTypes.Payload{},
			expectedLogs: nil,
			wantErr:      assert.NoError,
		},
		{
			name:         "Standard payload",
			faroPayload:  *PayloadFromFile(t, "payload.json"),
			expectedLogs: generateLogs(t),
			wantErr:      assert.NoError,
		},
		{
			name:         "Payload with browser brands as slice",
			faroPayload:  *PayloadFromFile(t, "payload-browser-brand-slice.json"),
			expectedLogs: generateLogsWithBrowserBrandsAsSlice(t),
			wantErr:      assert.NoError,
		},
		{
			name:         "Payload with browser brands as string",
			faroPayload:  *PayloadFromFile(t, "payload-browser-brand-string.json"),
			expectedLogs: generateLogsWithBrowserBrandsAsString(t),
			wantErr:      assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := TranslateToLogs(context.TODO(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			if tt.expectedLogs == nil && assert.Nil(t, logs) {
				return
			}

			assert.NoError(
				t,
				plogtest.CompareLogs(*tt.expectedLogs, *logs),
			)
		})
	}
}

func generateLogs(t *testing.T) *plog.Logs {
	t.Helper()
	logs := plog.NewLogs()
	rl := generateResourceLogs(t)
	lrs := generateLogRecords(t)
	lrs.CopyTo(rl.ScopeLogs().AppendEmpty().LogRecords())
	rl.CopyTo(logs.ResourceLogs().AppendEmpty())
	return &logs
}

func generateResourceLogs(t *testing.T) plog.ResourceLogs {
	t.Helper()
	resourceLogs := plog.NewResourceLogs()
	err := resourceLogs.Resource().Attributes().FromRaw(map[string]any{
		string(semconv.ServiceNameKey):           "testapp",
		string(semconv.ServiceVersionKey):        "abcdefg",
		string(semconv.ServiceNamespaceKey):      "testnamespace",
		string(semconv.DeploymentEnvironmentKey): "production",
		"app_bundle_id":                          "testBundleId",
	})
	require.NoError(t, err)
	return resourceLogs
}

func generateLogRecords(t *testing.T) plog.LogRecordSlice {
	t.Helper()
	records := plog.NewLogRecordSlice()

	logRecords := []struct {
		Body       string
		Attributes map[string]any
	}{
		{
			Body: "timestamp=2021-09-30T10:46:17.68Z kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar",
			Attributes: map[string]any{
				"kind": "log",
			},
		},
		{
			Body: "timestamp=2021-09-30T10:46:17.68Z kind=log message=\"loading price list\" level=trace context_component=AppRoot context_page=Pricing traceID=abcd spanID=ghj sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar",
			Attributes: map[string]any{
				"kind": "log",
			},
		},
		{
			Body: "timestamp=2021-09-30T10:46:17.68Z kind=exception type=Error value=\"Cannot read property 'find' of undefined\" stacktrace=\"Error: Cannot read property 'find' of undefined\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\\n  at dispatchAction (http://fe:3002/static/js/vendors~main.chunk.js:268095:9)\\n  at scheduleUpdateOnFiber (http://fe:3002/static/js/vendors~main.chunk.js:273726:13)\\n  at flushSyncCallbackQueue (http://fe:3002/static/js/vendors~main.chunk.js:263362:7)\\n  at flushSyncCallbackQueueImpl (http://fe:3002/static/js/vendors~main.chunk.js:263374:13)\\n  at runWithPriority$1 (http://fe:3002/static/js/vendors~main.chunk.js:263325:14)\\n  at unstable_runWithPriority (http://fe:3002/static/js/vendors~main.chunk.js:291265:16)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:263379:30)\\n  at performSyncWorkOnRoot (http://fe:3002/static/js/vendors~main.chunk.js:274126:22)\\n  at renderRootSync (http://fe:3002/static/js/vendors~main.chunk.js:274509:11)\\n  at workLoopSync (http://fe:3002/static/js/vendors~main.chunk.js:274543:9)\\n  at performUnitOfWork (http://fe:3002/static/js/vendors~main.chunk.js:274606:16)\\n  at beginWork$1 (http://fe:3002/static/js/vendors~main.chunk.js:275746:18)\\n  at beginWork (http://fe:3002/static/js/vendors~main.chunk.js:270944:20)\\n  at updateFunctionComponent (http://fe:3002/static/js/vendors~main.chunk.js:269291:24)\\n  at renderWithHooks (http://fe:3002/static/js/vendors~main.chunk.js:266969:22)\\n  at ? (http://fe:3002/static/js/main.chunk.js:2600:74)\\n  at useGetBooksQuery (http://fe:3002/static/js/main.chunk.js:1299:65)\\n  at Module.useQuery (http://fe:3002/static/js/vendors~main.chunk.js:8495:85)\\n  at useBaseQuery (http://fe:3002/static/js/vendors~main.chunk.js:8656:83)\\n  at useDeepMemo (http://fe:3002/static/js/vendors~main.chunk.js:8696:14)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8657:55)\\n  at QueryData.execute (http://fe:3002/static/js/vendors~main.chunk.js:7883:47)\\n  at QueryData.getExecuteResult (http://fe:3002/static/js/vendors~main.chunk.js:7944:23)\\n  at QueryData._this.getQueryResult (http://fe:3002/static/js/vendors~main.chunk.js:7790:19)\\n  at new ApolloError (http://fe:3002/static/js/vendors~main.chunk.js:5164:24)\" traceID=abcd spanID=def context_ReactError=\"Annoying Error\" context_component=ReactErrorBoundary sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar",
			Attributes: map[string]any{
				"kind": "exception",
				"hash": "2735541995122471342",
			},
		},
		{
			Body: "timestamp=2021-09-30T10:46:17.68Z kind=measurement type=\"page load\" context_hello=world ttfb=14.000000 ttfcp=22.120000 ttfp=20.120000 traceID=abcd spanID=def value_ttfb=14 value_ttfcp=22.12 value_ttfp=20.12 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar",
			Attributes: map[string]any{
				"kind": "measurement",
			},
		},
		{
			Body: "timestamp=2023-11-16T10:00:55.995Z kind=event event_name=faro.performanceEntry event_domain=browser event_data_connectEnd=3656 event_data_connectStart=337 event_data_decodedBodySize=0 event_data_domainLookupEnd=590 event_data_domainLookupStart=588 event_data_duration=3371 event_data_encodedBodySize=0 event_data_entryType=resource event_data_fetchStart=331 event_data_initiatorType=other event_data_name=https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/css/bootstrap.min.css.map event_data_nextHopProtocol=h2 event_data_redirectEnd=0 event_data_redirectStart=0 event_data_requestStart=3656 event_data_responseEnd=3702 event_data_responseStart=3690 event_data_secureConnectionStart=3638 event_data_serverTiming=[] event_data_startTime=331 event_data_transferSize=0 event_data_workerStart=0 sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=testuser user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar",
			Attributes: map[string]any{
				"kind": "event",
			},
		},
	}

	for _, logRecord := range logRecords {
		lr := records.AppendEmpty()
		lr.Body().SetStr(logRecord.Body)
		err := lr.Attributes().FromRaw(logRecord.Attributes)
		require.NoError(t, err)
	}

	return records
}

func generateLogsWithBrowserBrandsAsSlice(t *testing.T) *plog.Logs {
	t.Helper()
	logs := plog.NewLogs()
	rl := generateResourceLogs(t)
	lrs := generateLogRecordsWithBrowserBrandsAsSlice(t)
	lrs.CopyTo(rl.ScopeLogs().AppendEmpty().LogRecords())
	rl.CopyTo(logs.ResourceLogs().AppendEmpty())
	return &logs
}

func generateLogsWithBrowserBrandsAsString(t *testing.T) *plog.Logs {
	t.Helper()
	logs := plog.NewLogs()
	rl := generateResourceLogs(t)
	lrs := generateLogRecordsWithBrowserBrandsAsString(t)
	lrs.CopyTo(rl.ScopeLogs().AppendEmpty().LogRecords())
	rl.CopyTo(logs.ResourceLogs().AppendEmpty())
	return &logs
}

func generateLogRecordsWithBrowserBrandsAsSlice(t *testing.T) plog.LogRecordSlice {
	t.Helper()
	records := plog.NewLogRecordSlice()

	logRecords := []struct {
		Body       string
		Attributes map[string]any
	}{
		{
			Body: "timestamp=2023-11-16T10:00:55.995Z kind=event event_name=faro.performanceEntry event_domain=browser event_data_name=https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/css/bootstrap.min.css.map sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false browser_userAgent=\"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.12.1 Safari/537.36\" browser_language=en-US browser_viewportWidth=1920 browser_viewportHeight=1080 browser_brand_0_brand=brand1 browser_brand_0_version=1.0.0",
			Attributes: map[string]any{
				"kind": "event",
			},
		},
	}

	for _, logRecord := range logRecords {
		lr := records.AppendEmpty()
		lr.Body().SetStr(logRecord.Body)
		err := lr.Attributes().FromRaw(logRecord.Attributes)
		require.NoError(t, err)
	}

	return records
}

func generateLogRecordsWithBrowserBrandsAsString(t *testing.T) plog.LogRecordSlice {
	t.Helper()
	records := plog.NewLogRecordSlice()

	logRecords := []struct {
		Body       string
		Attributes map[string]any
	}{
		{
			Body: "timestamp=2023-11-16T10:00:55.995Z kind=event event_name=faro.performanceEntry event_domain=browser event_data_name=https://cdn.jsdelivr.net/npm/bootstrap@4.5.3/dist/css/bootstrap.min.css.map sdk_name=grafana-frontend-agent sdk_version=1.3.5 app_name=testapp app_namespace=testnamespace app_release=0.8.2 app_version=abcdefg app_environment=production browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false browser_userAgent=\"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.12.1 Safari/537.36\" browser_language=en-US browser_viewportWidth=1920 browser_viewportHeight=1080 browser_brands=\"Chromium;Google Inc.;\"",
			Attributes: map[string]any{
				"kind": "event",
			},
		},
	}

	for _, logRecord := range logRecords {
		lr := records.AppendEmpty()
		lr.Body().SetStr(logRecord.Body)
		err := lr.Attributes().FromRaw(logRecord.Attributes)
		require.NoError(t, err)
	}

	return records
}

func PayloadFromFile(t *testing.T, filename string) *faroTypes.Payload {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	var p faroTypes.Payload
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		t.Fatal(err)
	}

	return &p
}
