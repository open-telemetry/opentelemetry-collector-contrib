// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func createTracesAndSpan() (ptrace.Traces, ptrace.Span) {
	// make a trace
	traces := ptrace.NewTraces()
	// add trace attributes
	rs := traces.ResourceSpans().AppendEmpty()
	resource := rs.Resource()
	attrs := resource.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4) // service name + 3 attributes
	attrs.PutStr("service.name", "unittest-resource")
	attrs.PutStr("attr1", "unittest-foo")
	attrs.PutInt("attr2", 40)
	attrs.PutDouble("attr3", 3.14)

	// add a span
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	spans.EnsureCapacity(1)
	span := spans.AppendEmpty()
	// add span attributes
	span.SetTraceID(pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}))
	span.SetSpanID(pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
	span.SetName("unittest-span")
	startTime := pcommon.Timestamp(time.Now().UnixNano())
	span.SetStartTimestamp(startTime)
	span.SetEndTimestamp(startTime + 1)
	span.SetParentSpanID(pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	attrs = span.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-bar")
	attrs.PutInt("attr2", 41)
	attrs.PutDouble("attr3", 4.14)

	return traces, span
}

func TestAlertManagerExporterExtractSpanEvents(t *testing.T) {
	tests := []struct {
		name   string
		events int
	}{
		{"TestAlertManagerExporterExtractEvents0", 0},
		{"TestAlertManagerExporterExtractEvents1", 1},
		{"TestAlertManagerExporterExtractEvents5", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			set := exportertest.NewNopSettings(metadata.Type)
			am := newAlertManagerExporter(cfg, set.TelemetrySettings)
			require.NotNil(t, am)

			// make traces & a span
			traces, span := createTracesAndSpan()

			// add events
			for i := 0; i < tt.events; i++ {
				event := span.Events().AppendEmpty()
				// add event attributes
				startTime := pcommon.Timestamp(time.Now().UnixNano())
				event.SetTimestamp(startTime + 3)
				event.SetName(fmt.Sprintf("unittest-event-%d", i))
				attrs := event.Attributes()
				attrs.Clear()
				attrs.EnsureCapacity(4)
				attrs.PutStr("attr1", fmt.Sprintf("unittest-baz-%d", i))
				attrs.PutInt("attr2", 42)
				attrs.PutDouble("attr3", 5.14)
			}

			// test - events
			got := am.extractSpanEvents(traces)
			assert.Len(t, got, tt.events)
		})
	}
}

func TestAlertManagerExporterSpanEventNameAttributes(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// make traces & a span
	traces, span := createTracesAndSpan()

	// add a span event w/ 3 attributes
	event := span.Events().AppendEmpty()
	// add event attributes
	startTime := pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs := event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutInt("attr2", 42)
	attrs.PutDouble("attr3", 5.14)

	// test - 1 event
	got := am.extractSpanEvents(traces)

	// test - result length
	assert.Len(t, got, 1)

	// test - count of attributes
	assert.Equal(t, 3, got[0].spanEvent.Attributes().Len())
	attr, b := got[0].spanEvent.Attributes().Get("attr1")
	assert.True(t, b)
	assert.Equal(t, "unittest-event", got[0].spanEvent.Name())
	assert.Equal(t, "unittest-baz", attr.AsString())
	attr, b = got[0].spanEvent.Attributes().Get("attr3")
	assert.True(t, b)
	assert.Equal(t, 5.14, attr.Double())
}

func TestAlertManagerExporterSeverity(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SeverityAttribute = "foo"
	cfg.EventLabels = []string{}
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// make traces & a span
	traces, span := createTracesAndSpan()

	// add a span event with severity attribute
	event := span.Events().AppendEmpty()
	// add event attributes
	startTime := pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs := event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutStr("foo", "debug")

	// add a span event without severity attribute
	event = span.Events().AppendEmpty()
	// add event attributes
	startTime = pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs = event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutStr("bar", "debug")

	// add a span event with an empty severity attribute
	event = span.Events().AppendEmpty()
	startTime = pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs = event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("foo", "")

	// test - 0 event
	got := am.extractSpanEvents(traces)
	alerts := am.convertSpanEventsToAlertPayload(got)

	ls := model.LabelSet{"event_name": "unittest-event", "severity": "debug"}
	assert.Equal(t, ls, alerts[0].Labels)

	ls = model.LabelSet{"event_name": "unittest-event", "severity": "info"}
	assert.Equal(t, ls, alerts[1].Labels)

	assert.Equal(t, ls, alerts[2].Labels)
}

func TestAlertManagerExporterNoDefaultSeverity(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)
	cfg.EventLabels = []string{}
	// make traces & a span
	traces, span := createTracesAndSpan()

	// add a span event with severity attribute
	event := span.Events().AppendEmpty()
	// add event attributes
	startTime := pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs := event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutStr("attr2", "debug")

	// test - 0 event
	got := am.extractSpanEvents(traces)
	alerts := am.convertSpanEventsToAlertPayload(got)

	ls := model.LabelSet{"event_name": "unittest-event", "severity": "info"}
	assert.Equal(t, ls, alerts[0].Labels)
}

func TestAlertManagerExporterAlertPayload(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	cfg.EventLabels = []string{}

	require.NotNil(t, am)

	// make traces & a span
	_, span := createTracesAndSpan()

	// add a span event w/ 3 attributes
	event := span.Events().AppendEmpty()
	// add event attributes
	startTime := pcommon.Timestamp(time.Now().UTC().Unix())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs := event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutInt("attr2", 42)
	attrs.PutDouble("attr3", 5.14)

	var events []*alertmanagerEvent
	events = append(events, &alertmanagerEvent{
		spanEvent: event,
		severity:  am.defaultSeverity,
		traceID:   "0000000000000002",
		spanID:    "00000002",
	})

	got := am.convertSpanEventsToAlertPayload(events)

	// test - count of attributes
	expect := model.Alert{
		Labels:       model.LabelSet{"severity": "info", "event_name": "unittest-event"},
		Annotations:  model.LabelSet{"SpanID": "00000002", "TraceID": "0000000000000002", "attr1": "unittest-baz", "attr2": "42", "attr3": "5.14"},
		GeneratorURL: "opentelemetry-collector",
	}
	assert.Equal(t, expect.Labels, got[0].Labels)
	assert.Equal(t, expect.Annotations, got[0].Annotations)
	assert.Equal(t, expect.GeneratorURL, got[0].GeneratorURL)
}

func TestAlertManagerTracesExporterNoErrors(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	lte, err := newTracesExporter(t.Context(), cfg, exportertest.NewNopSettings(metadata.Type))
	fmt.Println(lte)
	require.NotNil(t, lte)
	assert.NoError(t, err)
}

func TestAlertManagerExporterSpanEventLabels(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// make traces & a span
	_, span := createTracesAndSpan()

	// add a span event w/ 3 attributes
	event := span.Events().AppendEmpty()
	// add event attributes
	startTime := pcommon.Timestamp(time.Now().UnixNano())
	event.SetTimestamp(startTime + 3)
	event.SetName("unittest-event")
	attrs := event.Attributes()
	attrs.Clear()
	attrs.EnsureCapacity(4)
	attrs.PutStr("attr1", "unittest-baz")
	attrs.PutInt("attr2", 42)
	attrs.PutDouble("attr3", 5.14)

	var events []*alertmanagerEvent
	events = append(events, &alertmanagerEvent{
		spanEvent: event,
		severity:  am.defaultSeverity,
		traceID:   "0000000000000002",
		spanID:    "00000002",
	})

	got := am.convertSpanEventsToAlertPayload(events)

	// test - count of attributes
	expect := model.Alert{
		Labels:       model.LabelSet{"severity": "info", "event_name": "unittest-event", "attr1": "unittest-baz", "attr2": "42"},
		Annotations:  model.LabelSet{"SpanID": "00000002", "TraceID": "0000000000000002", "attr1": "unittest-baz", "attr2": "42", "attr3": "5.14"},
		GeneratorURL: "opentelemetry-collector",
	}
	assert.Equal(t, expect.Labels, got[0].Labels)
	assert.Equal(t, expect.Annotations, got[0].Annotations)
	assert.Equal(t, expect.GeneratorURL, got[0].GeneratorURL)
}

type mockServer struct {
	mockserver            *httptest.Server // this means mockServer aggregates 'httptest.Server', but can it's more like inheritance in C++
	fooCalledSuccessfully bool             // this is false by default
}

func newMockServer(t *testing.T) *mockServer {
	mock := mockServer{
		fooCalledSuccessfully: false,
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		_, errWrite := fmt.Fprint(w, "test")
		assert.NoError(t, errWrite)
		_, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		mock.fooCalledSuccessfully = true
		_, _ = w.Write([]byte("hello"))
	})
	mock.mockserver = httptest.NewServer(handler)
	return &mock
}

func TestAlertManagerPostAlert(t *testing.T) {
	mock := newMockServer(t)
	defer func() { mock.mockserver.Close() }()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	var alerts []model.Alert
	alerts = append(alerts, model.Alert{
		Labels:       model.LabelSet{"new": "info"},
		Annotations:  model.LabelSet{"foo": "bar1"},
		GeneratorURL: "http://example.com/alert",
	})

	cfg.Endpoint = mock.mockserver.URL
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	err := am.start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	err = am.postAlert(t.Context(), alerts)
	assert.Contains(t, err.Error(), "failed - \"404 Not Found\"")

	cfg.APIVersion = "v1"
	am = newAlertManagerExporter(cfg, set.TelemetrySettings)
	err = am.start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	err = am.postAlert(t.Context(), alerts)
	assert.NoError(t, err)

	assert.True(t, mock.fooCalledSuccessfully, "mock server wasn't called")
}

func TestClientConfig(t *testing.T) {
	endpoint := "http://" + testutil.GetAvailableLocalAddress(t)
	fmt.Println(endpoint)

	useSecureClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	useSecureClientConfig.MaxIdleConns = 0
	useSecureClientConfig.IdleConnTimeout = 0
	useSecureClientConfig.ForceAttemptHTTP2 = false
	useSecureClientConfig.Endpoint = endpoint
	useSecureClientConfig.TLS = configtls.ClientConfig{
		Insecure: false,
	}

	headersClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	headersClientConfig.MaxIdleConns = 0
	headersClientConfig.IdleConnTimeout = 0
	headersClientConfig.ForceAttemptHTTP2 = false
	headersClientConfig.Endpoint = endpoint
	headersClientConfig.Headers = configopaque.MapList{
		{Name: "hdr1", Value: "val1"},
		{Name: "hdr2", Value: "val2"},
	}

	caCertClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	caCertClientConfig.MaxIdleConns = 0
	caCertClientConfig.IdleConnTimeout = 0
	caCertClientConfig.ForceAttemptHTTP2 = false
	caCertClientConfig.Endpoint = endpoint
	caCertClientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "testdata/test_cert.pem",
		},
	}

	certPemFileErrorClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	certPemFileErrorClientConfig.MaxIdleConns = 0
	certPemFileErrorClientConfig.IdleConnTimeout = 0
	certPemFileErrorClientConfig.ForceAttemptHTTP2 = false
	certPemFileErrorClientConfig.Endpoint = endpoint
	certPemFileErrorClientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "nosuchfile",
		},
	}

	tests := []struct {
		name             string
		config           *Config
		mustFailOnCreate bool
		mustFailOnStart  bool
	}{
		{
			name: "UseSecure",
			config: &Config{
				ClientConfig: useSecureClientConfig,
			},
		},
		{
			name: "Headers",
			config: &Config{
				ClientConfig: headersClientConfig,
			},
		},
		{
			name: "CaCert",
			config: &Config{
				ClientConfig: caCertClientConfig,
			},
		},
		{
			name: "CertPemFileError",
			config: &Config{
				ClientConfig: certPemFileErrorClientConfig,
			},
			mustFailOnCreate: false,
			mustFailOnStart:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := exportertest.NewNopSettings(metadata.Type)
			am := newAlertManagerExporter(tt.config, set.TelemetrySettings)

			exp, err := newTracesExporter(t.Context(), tt.config, set)
			if tt.mustFailOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, exp)

			err = am.start(t.Context(), componenttest.NewNopHost())
			if tt.mustFailOnStart {
				assert.Error(t, err)
			}

			t.Cleanup(func() {
				require.NoError(t, am.shutdown(t.Context()))
			})
		})
	}
}

func TestConvertSpanEventSliceToArray(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DefaultSeverity = "info"
	cfg.SeverityAttribute = "severity"
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	eventSlice := ptrace.NewSpanEventSlice()
	event := eventSlice.AppendEmpty()
	event.SetName("test-event")
	event.Attributes().PutStr("severity", "debug")
	event.Attributes().PutStr("key", "value")

	events := am.convertSpanEventSliceToArray(eventSlice, traceID, spanID)
	require.Len(t, events, 1)
	assert.Equal(t, "test-event", events[0].spanEvent.Name())
	assert.Equal(t, "debug", events[0].severity)
	assert.Equal(t, traceID.String(), events[0].traceID)
	assert.Equal(t, spanID.String(), events[0].spanID)
}

// Logs Testing

// It checks the handling of TraceID, SpanID, severity attributes, SeverityText, and default severity.
func TestConvertLogRecordSliceToArray(t *testing.T) {
	// Setup
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DefaultSeverity = "info"
	cfg.SeverityAttribute = "severity"
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// Create Logs with multiple log records
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	// 1. Log with TraceID, SpanID, and Severity
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	logRecord.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	logRecord.Attributes().PutStr("severity", "error")
	logRecord.Body().SetStr("Log 1")

	// 2. Log without TraceID, SpanID, and with severity
	logRecord2 := scopeLogs.LogRecords().AppendEmpty()
	logRecord2.SetTraceID(pcommon.TraceID([16]byte{})) // empty TraceID
	logRecord2.SetSpanID(pcommon.SpanID([8]byte{}))    // empty SpanID
	logRecord2.Attributes().PutStr("severity", "warning")
	logRecord2.Body().SetStr("Log 2")

	// 3. Log without TraceID, SpanID, and severity attribute but with native SeverityText
	logRecord3 := scopeLogs.LogRecords().AppendEmpty()
	logRecord3.SetTraceID(pcommon.TraceID([16]byte{})) // empty TraceID
	logRecord3.SetSpanID(pcommon.SpanID([8]byte{}))    // empty SpanID
	logRecord3.SetSeverityText("critical")
	logRecord3.Body().SetStr("Log 3")

	// 4. Log without TraceID, SpanID, severity attribute, or SeverityText (default should be used)
	logRecord4 := scopeLogs.LogRecords().AppendEmpty()
	logRecord4.SetTraceID(pcommon.TraceID([16]byte{})) // empty TraceID
	logRecord4.SetSpanID(pcommon.SpanID([8]byte{}))    // empty SpanID
	logRecord4.Body().SetStr("Log 4")

	// 5. Log with empty severity attribute and SeverityText (SeverityText should be used)
	logRecord5 := scopeLogs.LogRecords().AppendEmpty()
	logRecord5.Attributes().PutStr("severity", "")
	logRecord5.SetSeverityText("critical")
	logRecord5.Body().SetStr("Log 5")

	// 6. Log with empty severity attribute and no SeverityText (default should be used)
	logRecord6 := scopeLogs.LogRecords().AppendEmpty()
	logRecord6.Attributes().PutStr("severity", "")
	logRecord6.Body().SetStr("Log 6")

	// Run the method
	events := am.convertLogRecordSliceToArray(scopeLogs.LogRecords())

	// Assertions
	require.Len(t, events, 6)

	// Check the first event (log 1)
	event1 := events[0]
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", event1.traceID)
	assert.Equal(t, "0102030405060708", event1.spanID)
	assert.Equal(t, "error", event1.severity)
	assert.Equal(t, "Log 1", event1.logRecord.Body().Str())

	// Check the second event (log 2)
	event2 := events[1]
	assert.Empty(t, event2.traceID, "TraceID should be empty")
	assert.Empty(t, event2.spanID, "SpanID should be empty")
	assert.Equal(t, "warning", event2.severity)
	assert.Equal(t, "Log 2", event2.logRecord.Body().Str())

	// Check the third event (log 3)
	event3 := events[2]
	assert.Empty(t, event3.traceID, "TraceID should be empty")
	assert.Empty(t, event3.spanID, "SpanID should be empty")
	assert.Equal(t, "critical", event3.severity)
	assert.Equal(t, "Log 3", event3.logRecord.Body().Str())

	// Check the fourth event (log 4)
	event4 := events[3]
	assert.Empty(t, event4.traceID, "TraceID should be empty")
	assert.Empty(t, event4.spanID, "SpanID should be empty")
	assert.Equal(t, "info", event4.severity) // Default severity
	assert.Equal(t, "Log 4", event4.logRecord.Body().Str())

	event5 := events[4]
	assert.Equal(t, "critical", event5.severity)
	assert.Equal(t, "Log 5", event5.logRecord.Body().Str())

	event6 := events[5]
	assert.Equal(t, "info", event6.severity)
	assert.Equal(t, "Log 6", event6.logRecord.Body().Str())
}

func TestExtractLogEvents(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DefaultSeverity = "info"
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// Construct Logs with two ResourceLogs
	logs := plog.NewLogs()

	// Valid ResourceLogs
	validRL := logs.ResourceLogs().AppendEmpty()
	validRL.Resource().Attributes().PutStr("resource_key", "resource_value")
	validSL := validRL.ScopeLogs().AppendEmpty()
	logRecord := validSL.LogRecords().AppendEmpty()
	logRecord.Attributes().PutStr("env", "prod")
	logRecord.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	logRecord.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	logRecord.Body().SetStr("Test log")

	// Run extractor
	events := am.extractLogEvents(logs)

	// Assertions
	require.Len(t, events, 1)
	event := events[0]
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", event.traceID)
	assert.Equal(t, "0102030405060708", event.spanID)
	assert.Equal(t, "info", event.severity)
	attr, ok := event.logRecord.Attributes().Get("env")
	require.True(t, ok)
	assert.Equal(t, "prod", attr.Str())
}

func TestCreateLogAnnotations(t *testing.T) {
	// Setup
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DefaultSeverity = "info"
	set := exportertest.NewNopSettings(metadata.Type)
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	require.NotNil(t, am)

	// Create a log record with TraceID, SpanID, Body, and Timestamp
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	// 1. Log with TraceID, SpanID, Body and Timestamp
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	logRecord.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	logRecord.Body().SetStr("Test log body")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Create the alertmanagerLogEvent
	event := &alertmanagerLogEvent{
		logRecord: logRecord,
		traceID:   logRecord.TraceID().String(),
		spanID:    logRecord.SpanID().String(),
		severity:  "info",
	}

	// Run the real function
	labelSet := createLogAnnotations(event)

	// Assertions
	// Check if the LabelSet has TraceID, SpanID, Body and Timestamp
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", string(labelSet["TraceID"]))
	assert.Equal(t, "0102030405060708", string(labelSet["SpanID"]))
	assert.Equal(t, "Test log body", string(labelSet["Body"]))

	// 2. Log without TraceID and SpanID
	logRecord2 := scopeLogs.LogRecords().AppendEmpty()
	logRecord2.SetTraceID(pcommon.TraceID([16]byte{})) // empty TraceID
	logRecord2.SetSpanID(pcommon.SpanID([8]byte{}))    // empty SpanID
	logRecord2.Body().SetStr("Log without Trace and Span")
	event2 := &alertmanagerLogEvent{
		logRecord: logRecord2,
		traceID:   logRecord2.TraceID().String(),
		spanID:    logRecord2.SpanID().String(),
		severity:  "info",
	}

	// Run the real function for event2
	labelSet2 := createLogAnnotations(event2)

	// Assertions for log without TraceID and SpanID
	assert.NotContains(t, labelSet2, "TraceID", "TraceID should not be present")
	assert.NotContains(t, labelSet2, "SpanID", "SpanID should not be present")
	assert.Equal(t, "Log without Trace and Span", string(labelSet2["Body"]))
}

func TestCreateLogLabels(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.EventLabels = []string{"env", "service"} // only include these
	cfg.DefaultSeverity = "info"
	set := exportertest.NewNopSettings(metadata.Type)

	exporter := newAlertManagerExporter(cfg, set.TelemetrySettings)

	// Create a log record
	logRecord := plog.NewLogRecord()
	logRecord.Attributes().PutStr("env", "prod")
	logRecord.Attributes().PutStr("service", "auth-service")
	logRecord.Attributes().PutStr("ignored", "should-not-appear")
	logRecord.Attributes().PutStr("event.name", "ERROR")

	event := &alertmanagerLogEvent{
		logRecord: logRecord,
		severity:  "error",
		traceID:   "",
		spanID:    "",
	}

	labels := exporter.createLogLabels(event)

	// Check included labels
	assert.Equal(t, model.LabelValue("prod"), labels["env"])
	assert.Equal(t, model.LabelValue("auth-service"), labels["service"])
	assert.Equal(t, model.LabelValue("error"), labels["severity"])
	assert.Equal(t, model.LabelValue("ERROR"), labels["event_name"])

	// Ensure excluded label is not present
	_, exists := labels["ignored"]
	assert.False(t, exists, "Label 'ignored' should not be included")

	logRecordWithoutEventName := plog.NewLogRecord()
	eventWithoutEventName := &alertmanagerLogEvent{
		logRecord: logRecordWithoutEventName,
		severity:  "error",
	}

	labels = exporter.createLogLabels(eventWithoutEventName)
	assert.Equal(t, model.LabelValue("log_record"), labels["event_name"])

	logRecordWithEmptyEventName := plog.NewLogRecord()
	logRecordWithEmptyEventName.Attributes().PutStr("event.name", "")
	eventWithEmptyEventName := &alertmanagerLogEvent{
		logRecord: logRecordWithEmptyEventName,
		severity:  "error",
	}

	labels = exporter.createLogLabels(eventWithEmptyEventName)
	assert.Equal(t, model.LabelValue("log_record"), labels["event_name"])
}

func TestNewLogsExporter(t *testing.T) {
	// Create default config
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.DefaultSeverity = "info"
	cfg.SeverityAttribute = "severity"

	// Dummy exporter settings
	set := exportertest.NewNopSettings(metadata.Type)

	// Create the exporter
	exp, err := newLogsExporter(t.Context(), cfg, set)

	// Assertions
	require.NoError(t, err, "expected no error creating exporter")
	require.NotNil(t, exp, "exporter should not be nil")
}

func TestSpanEventNameAttributeToLabelConversion(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.EventLabels = []string{}
	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newAlertManagerExporter(cfg, set.TelemetrySettings)

	eventSlice := ptrace.NewSpanEventSlice()
	spanEvent := eventSlice.AppendEmpty()
	spanEvent.SetName("span-event-name")
	spanEvent.Attributes().PutStr("event.name", "attribute-event-name")

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	events := exporter.convertSpanEventSliceToArray(eventSlice, traceID, spanID)
	require.Len(t, events, 1)

	labels := exporter.createTraceLabels(events[0])
	annotations := createTraceAnnotations(events[0])

	assert.Equal(t, model.LabelValue("span-event-name"), labels["event_name"],
		"event_name label should use spanEvent.Name() field")
	assert.Equal(t, model.LabelValue("attribute-event-name"), annotations["event_name"],
		"event.name attribute should be converted to event_name annotation")
	assert.NotContains(t, annotations, model.LabelName("event.name"))
	assert.Equal(t, model.LabelValue("info"), labels["severity"])
}

func TestLogEventNameAttributeConversion(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.EventLabels = []string{"event.name"}
	cfg.DefaultSeverity = "info"
	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newAlertManagerExporter(cfg, set.TelemetrySettings)

	logRecord := plog.NewLogRecord()
	logRecord.Attributes().PutStr("event.name", "log-event-value")
	logRecord.Body().SetStr("test log")

	event := &alertmanagerLogEvent{
		logRecord: logRecord,
		severity:  "error",
		traceID:   "",
		spanID:    "",
	}

	labels := exporter.createLogLabels(event)
	annotations := createLogAnnotations(event)

	assert.Equal(t, model.LabelValue("log-event-value"), labels["event_name"],
		"event.name attribute should be converted to event_name label")
	assert.Equal(t, model.LabelValue("log-event-value"), annotations["event_name"],
		"event.name attribute should be converted to event_name annotation")
	assert.NotContains(t, labels, model.LabelName("event.name"))
	assert.NotContains(t, annotations, model.LabelName("event.name"))
}
