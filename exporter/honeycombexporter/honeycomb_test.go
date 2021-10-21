// Copyright 2020 OpenTelemetry Authors
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

package honeycombexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/honeycombio/libhoney-go/transmission"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type honeycombData struct {
	Data map[string]interface{} `json:"data"`
}

func testingServer(callback func(data []honeycombData)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		uncompressed, err := zstd.NewReader(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		defer req.Body.Close()
		b, err := ioutil.ReadAll(uncompressed)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		var data []honeycombData
		err = json.Unmarshal(b, &data)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		callback(data)
		rw.Write([]byte(`OK`))
	}))
}

func testTracesExporter(td pdata.Traces, t *testing.T, cfg *Config) []honeycombData {
	var got []honeycombData
	server := testingServer(func(data []honeycombData) {
		got = append(got, data...)
	})
	defer server.Close()

	cfg.APIURL = server.URL

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := createTracesExporter(context.Background(), params, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	require.NoError(t, err)
	exporter.Shutdown(context.Background())

	return got
}

func baseConfig() *Config {
	return &Config{
		ExporterSettings:    config.NewExporterSettings(config.NewComponentID(typeStr)),
		APIKey:              "test",
		Dataset:             "test",
		Debug:               false,
		SampleRateAttribute: "",
	}
}

func TestExporter(t *testing.T) {
	td := pdata.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"service.name": pdata.NewAttributeValueString("test_service"),
		"A":            pdata.NewAttributeValueString("B"),
		"B":            pdata.NewAttributeValueString("C"),
	})
	instrLibrarySpans := rs.InstrumentationLibrarySpans().AppendEmpty()
	lib := instrLibrarySpans.InstrumentationLibrary()
	lib.SetName("my.custom.library")
	lib.SetVersion("1.0.0")

	clientSpan := instrLibrarySpans.Spans().AppendEmpty()
	clientSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	clientSpan.SetSpanID(pdata.NewSpanID([8]byte{0x03}))
	clientSpan.SetParentSpanID(pdata.NewSpanID([8]byte{0x02}))
	clientSpan.SetName("client")
	clientSpan.SetKind(pdata.SpanKindClient)
	clientSpanLink := clientSpan.Links().AppendEmpty()
	clientSpanLink.SetTraceID(pdata.NewTraceID([16]byte{0x04}))
	clientSpanLink.SetSpanID(pdata.NewSpanID([8]byte{0x05}))
	clientSpanLink.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"span_link_attr": pdata.NewAttributeValueInt(12345),
	})

	serverSpan := instrLibrarySpans.Spans().AppendEmpty()
	serverSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	serverSpan.SetSpanID(pdata.NewSpanID([8]byte{0x04}))
	serverSpan.SetParentSpanID(pdata.NewSpanID([8]byte{0x03}))
	serverSpan.SetName("server")
	serverSpan.SetKind(pdata.SpanKindServer)

	rootSpan := instrLibrarySpans.Spans().AppendEmpty()
	rootSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	rootSpan.SetSpanID(pdata.NewSpanID([8]byte{0x02}))
	rootSpan.SetName("root")
	rootSpan.SetKind(pdata.SpanKindServer)
	rootSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"span_attr_name": pdata.NewAttributeValueString("Span Attribute"),
		"B":              pdata.NewAttributeValueString("D"),
	})
	rootSpanEvent := rootSpan.Events().AppendEmpty()
	rootSpanEvent.SetTimestamp(0)
	rootSpanEvent.SetName("Some Description")
	rootSpanEvent.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"attribute_name": pdata.NewAttributeValueString("Hello MessageEvent"),
		"B":              pdata.NewAttributeValueString("D"),
	})

	got := testTracesExporter(td, t, baseConfig())
	want := []honeycombData{
		{
			Data: map[string]interface{}{
				"meta.annotation_type": "link",
				"span_link_attr":       float64(12345),
				"trace.trace_id":       "01000000000000000000000000000000",
				"trace.parent_id":      "0300000000000000",
				"trace.link.span_id":   "0500000000000000",
				"trace.link.trace_id":  "04000000000000000000000000000000",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":     float64(0),
				"name":            "client",
				"service.name":    "test_service",
				"span_kind":       "client",
				"status.code":     float64(0), // Default status code
				"status.message":  "STATUS_CODE_UNSET",
				"trace.parent_id": "0200000000000000",
				"trace.span_id":   "0300000000000000",
				"trace.trace_id":  "01000000000000000000000000000000",
				"library.name":    "my.custom.library",
				"library.version": "1.0.0",
				"A":               "B",
				"B":               "C",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":     float64(0),
				"name":            "server",
				"service.name":    "test_service",
				"span_kind":       "server",
				"status.code":     float64(0), // Default status code
				"status.message":  "STATUS_CODE_UNSET",
				"trace.parent_id": "0300000000000000",
				"trace.span_id":   "0400000000000000",
				"trace.trace_id":  "01000000000000000000000000000000",
				"library.name":    "my.custom.library",
				"library.version": "1.0.0",
				"A":               "B",
				"B":               "C",
			},
		},
		{
			Data: map[string]interface{}{
				"A":                    "B",
				"B":                    "D",
				"attribute_name":       "Hello MessageEvent",
				"meta.annotation_type": "span_event",
				"name":                 "Some Description",
				"service.name":         "test_service",
				"trace.parent_id":      "0200000000000000",
				"trace.parent_name":    "root",
				"trace.trace_id":       "01000000000000000000000000000000",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":     float64(0),
				"name":            "root",
				"service.name":    "test_service",
				"span_attr_name":  "Span Attribute",
				"span_kind":       "server",
				"status.code":     float64(0), // Default status code
				"status.message":  "STATUS_CODE_UNSET",
				"trace.span_id":   "0200000000000000",
				"trace.trace_id":  "01000000000000000000000000000000",
				"A":               "B",
				"B":               "D",
				"library.name":    "my.custom.library",
				"library.version": "1.0.0",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}

func TestSpanKinds(t *testing.T) {
	td := pdata.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"service.name": pdata.NewAttributeValueString("test_service"),
	})
	instrLibrarySpans := rs.InstrumentationLibrarySpans().AppendEmpty()
	lib := instrLibrarySpans.InstrumentationLibrary()
	lib.SetName("my.custom.library")
	lib.SetVersion("1.0.0")

	initSpan(instrLibrarySpans.Spans().AppendEmpty())

	spanKinds := []pdata.SpanKind{
		pdata.SpanKindInternal,
		pdata.SpanKindClient,
		pdata.SpanKindServer,
		pdata.SpanKindProducer,
		pdata.SpanKindConsumer,
		pdata.SpanKindUnspecified,
		pdata.SpanKind(1000),
	}

	expectedStrings := []string{
		"internal",
		"client",
		"server",
		"producer",
		"consumer",
		"unspecified",
		"unspecified",
	}

	for idx, kind := range spanKinds {
		stringKind := expectedStrings[idx]
		t.Run(fmt.Sprintf("span kind %s", stringKind), func(t *testing.T) {
			want := []honeycombData{
				{
					Data: map[string]interface{}{
						"duration_ms":     float64(0),
						"name":            "spanName",
						"library.name":    "my.custom.library",
						"library.version": "1.0.0",
						"service.name":    "test_service",
						"span_attr_name":  "Span Attribute",
						"span_kind":       stringKind,
						"status.code":     float64(0), // Default status code
						"status.message":  "STATUS_CODE_UNSET",
						"trace.span_id":   "0300000000000000",
						"trace.parent_id": "0200000000000000",
						"trace.trace_id":  "01000000000000000000000000000000",
					},
				},
			}

			instrLibrarySpans.Spans().At(0).SetKind(kind)

			got := testTracesExporter(td, t, baseConfig())

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("otel span: (-want +got):\n%s", diff)
			}
		})
	}
}

func initSpan(span pdata.Span) {
	span.SetName("spanName")
	span.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	span.SetParentSpanID(pdata.NewSpanID([8]byte{0x02}))
	span.SetSpanID(pdata.NewSpanID([8]byte{0x03}))

	span.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"span_attr_name": pdata.NewAttributeValueString("Span Attribute"),
	})
}

func TestSampleRateAttribute(t *testing.T) {
	td := pdata.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{})
	instrLibrarySpans := rs.InstrumentationLibrarySpans().AppendEmpty()

	intSampleRateSpan := instrLibrarySpans.Spans().AppendEmpty()
	intSampleRateSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	intSampleRateSpan.SetSpanID(pdata.NewSpanID([8]byte{0x02}))
	intSampleRateSpan.SetName("root")
	intSampleRateSpan.SetKind(pdata.SpanKindServer)
	intSampleRateSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"some_attribute": pdata.NewAttributeValueString("A value"),
		"hc.sample.rate": pdata.NewAttributeValueInt(13),
	})

	noSampleRateSpan := instrLibrarySpans.Spans().AppendEmpty()
	noSampleRateSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	noSampleRateSpan.SetSpanID(pdata.NewSpanID([8]byte{0x02}))
	noSampleRateSpan.SetName("root")
	noSampleRateSpan.SetKind(pdata.SpanKindServer)
	noSampleRateSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"no_sample_rate": pdata.NewAttributeValueString("gets_default"),
	})

	invalidSampleRateSpan := instrLibrarySpans.Spans().AppendEmpty()
	invalidSampleRateSpan.SetTraceID(pdata.NewTraceID([16]byte{0x01}))
	invalidSampleRateSpan.SetSpanID(pdata.NewSpanID([8]byte{0x02}))
	invalidSampleRateSpan.SetName("root")
	invalidSampleRateSpan.SetKind(pdata.SpanKindServer)
	invalidSampleRateSpan.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"hc.sample.rate": pdata.NewAttributeValueString("wrong_type"),
	})

	cfg := baseConfig()
	cfg.SampleRateAttribute = "hc.sample.rate"

	got := testTracesExporter(td, t, cfg)

	want := []honeycombData{
		{
			Data: map[string]interface{}{
				"duration_ms":    float64(0),
				"hc.sample.rate": float64(13),
				"name":           "root",
				"span_kind":      "server",
				"status.code":    float64(0), // Default status code
				"status.message": "STATUS_CODE_UNSET",
				"trace.span_id":  "0200000000000000",
				"trace.trace_id": "01000000000000000000000000000000",
				"some_attribute": "A value",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":    float64(0),
				"name":           "root",
				"span_kind":      "server",
				"status.code":    float64(0), // Default status code
				"status.message": "STATUS_CODE_UNSET",
				"trace.span_id":  "0200000000000000",
				"trace.trace_id": "01000000000000000000000000000000",
				"no_sample_rate": "gets_default",
			},
		},
		{
			Data: map[string]interface{}{
				"duration_ms":    float64(0),
				"hc.sample.rate": "wrong_type",
				"name":           "root",
				"span_kind":      "server",
				"status.code":    float64(0), // Default status code
				"status.message": "STATUS_CODE_UNSET",
				"trace.span_id":  "0200000000000000",
				"trace.trace_id": "01000000000000000000000000000000",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("otel span: (-want +got):\n%s", diff)
	}
}

func TestRunErrorLogger_OnError(t *testing.T) {
	obs, logs := observer.New(zap.WarnLevel)
	logger := zap.New(obs)

	cfg := createDefaultConfig().(*Config)
	exporter, err := newHoneycombTracesExporter(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()
	defer ctx.Done()

	channel := make(chan transmission.Response)

	go func() {
		channel <- transmission.Response{
			Err: errors.New("its a transmission error"),
		}
		close(channel)
	}()

	exporter.RunErrorLogger(ctx, channel)

	expectedLogs := []observer.LoggedEntry{{
		Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "its a transmission error"},
		Context: []zapcore.Field{},
	}}

	assert.Equal(t, 1, logs.Len())
	assert.Equal(t, expectedLogs, logs.AllUntimed())
}

func TestDebugUsesDebugLogger(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Debug = true
	_, err := newHoneycombTracesExporter(cfg, zap.NewNop())
	require.NoError(t, err)
}
