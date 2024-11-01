// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter

import (
	"context"
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
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

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
	attrs.PutStr(conventions.AttributeServiceName, "unittest-resource")
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

func TestAlertManagerExporterExtractEvents(t *testing.T) {
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
			set := exportertest.NewNopSettings()
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
			got := am.extractEvents(traces)
			assert.Len(t, got, tt.events)
		})
	}
}

func TestAlertManagerExporterEventNameAttributes(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings()
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
	got := am.extractEvents(traces)

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
	set := exportertest.NewNopSettings()
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

	// test - 0 event
	got := am.extractEvents(traces)
	alerts := am.convertEventsToAlertPayload(got)

	ls := model.LabelSet{"event_name": "unittest-event", "severity": "debug"}
	assert.Equal(t, ls, alerts[0].Labels)

	ls = model.LabelSet{"event_name": "unittest-event", "severity": "info"}
	assert.Equal(t, ls, alerts[1].Labels)
}

func TestAlertManagerExporterNoDefaultSeverity(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings()
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
	attrs.PutStr("attr2", "debug")

	// test - 0 event
	got := am.extractEvents(traces)
	alerts := am.convertEventsToAlertPayload(got)

	ls := model.LabelSet{"event_name": "unittest-event", "severity": "info"}
	assert.Equal(t, ls, alerts[0].Labels)
}

func TestAlertManagerExporterAlertPayload(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := exportertest.NewNopSettings()
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)

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

	got := am.convertEventsToAlertPayload(events)

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
	lte, err := newTracesExporter(context.Background(), cfg, exportertest.NewNopSettings())
	fmt.Println(lte)
	require.NotNil(t, lte)
	assert.NoError(t, err)
}

type (
	MockServer struct {
		mockserver            *httptest.Server // this means MockServer aggreagates 'httptest.Server', but can it's more like inheritance in C++
		fooCalledSuccessfully bool             // this is false by default
	}
)

func newMockServer(t *testing.T) *MockServer {
	mock := MockServer{
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
	set := exportertest.NewNopSettings()
	am := newAlertManagerExporter(cfg, set.TelemetrySettings)
	err := am.start(context.Background(), componenttest.NewNopHost())

	assert.NoError(t, err)

	err = am.postAlert(context.Background(), alerts)
	assert.NoError(t, err)
	assert.True(t, mock.fooCalledSuccessfully, "mock server wasn't called")
}

func TestClientConfig(t *testing.T) {
	endpoint := "http://" + testutil.GetAvailableLocalAddress(t)
	fmt.Println(endpoint)
	tests := []struct {
		name             string
		config           *Config
		mustFailOnCreate bool
		mustFailOnStart  bool
	}{
		{
			name: "UseSecure",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Insecure: false,
					},
				},
			},
		},
		{
			name: "Headers",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: endpoint,
					Headers: map[string]configopaque.String{
						"hdr1": "val1",
						"hdr2": "val2",
					},
				},
			},
		},
		{
			name: "CaCert",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "testdata/test_cert.pem",
						},
					},
				},
			},
		},
		{
			name: "CertPemFileError",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: endpoint,
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile: "nosuchfile",
						},
					},
				},
			},
			mustFailOnCreate: false,
			mustFailOnStart:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := exportertest.NewNopSettings()
			am := newAlertManagerExporter(tt.config, set.TelemetrySettings)

			exp, err := newTracesExporter(context.Background(), tt.config, set)
			if tt.mustFailOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, exp)

			err = am.start(context.Background(), componenttest.NewNopHost())
			if tt.mustFailOnStart {
				assert.Error(t, err)
			}

			t.Cleanup(func() {
				require.NoError(t, am.shutdown(context.Background()))
			})
		})
	}
}
