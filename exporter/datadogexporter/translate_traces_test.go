// Copyright The OpenTelemetry Authors
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

package datadogexporter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/stats"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
)

func NewResourceSpansData(mockTraceID [16]byte, mockSpanID [8]byte, mockParentSpanID [8]byte, statusCode pdata.StatusCode, resourceEnvAndService bool) pdata.ResourceSpans {
	// The goal of this test is to ensure that each span in
	// pdata.ResourceSpans is transformed to its *trace.SpanData correctly!

	endTime := time.Now().Round(time.Second)
	pdataEndTime := pdata.TimestampUnixNano(endTime.UnixNano())
	startTime := endTime.Add(-90 * time.Second)
	pdataStartTime := pdata.TimestampUnixNano(startTime.UnixNano())

	rs := pdata.NewResourceSpans()
	ilss := rs.InstrumentationLibrarySpans()
	ilss.Resize(1)
	il := ilss.At(0).InstrumentationLibrary()
	il.SetName("test_il_name")
	il.SetVersion("test_il_version")
	spans := ilss.At(0).Spans()
	spans.Resize(1)

	span := spans.At(0)
	traceID := pdata.NewTraceID(mockTraceID)
	spanID := pdata.NewSpanID(mockSpanID)
	parentSpanID := pdata.NewSpanID(mockParentSpanID)
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName("End-To-End Here")
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdataStartTime)
	span.SetEndTime(pdataEndTime)
	span.SetTraceState("tracestatekey=tracestatevalue")

	status := span.Status()
	if statusCode == pdata.StatusCodeError {
		status.SetCode(pdata.StatusCodeError)
		status.SetMessage("This is not a drill!")
	} else {
		status.SetCode(statusCode)
	}

	events := span.Events()
	events.Resize(2)

	events.At(0).SetTimestamp(pdataStartTime)
	events.At(0).SetName("start")

	events.At(1).SetTimestamp(pdataEndTime)
	events.At(1).SetName("end")
	events.At(1).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	attribs := map[string]pdata.AttributeValue{
		"cache_hit":  pdata.NewAttributeValueBool(true),
		"timeout_ns": pdata.NewAttributeValueInt(12e9),
		"ping_count": pdata.NewAttributeValueInt(25),
		"agent":      pdata.NewAttributeValueString("ocagent"),
	}

	if statusCode == pdata.StatusCodeError {
		attribs["http.status_code"] = pdata.NewAttributeValueString("501")
	}

	span.Attributes().InitFromMap(attribs)

	resource := rs.Resource()

	if resourceEnvAndService {
		resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			"namespace":              pdata.NewAttributeValueString("kube-system"),
			"service.name":           pdata.NewAttributeValueString("test-resource-service-name"),
			"deployment.environment": pdata.NewAttributeValueString("test-env"),
			"service.version":        pdata.NewAttributeValueString("test-version"),
		})

	} else {
		resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			"namespace": pdata.NewAttributeValueString("kube-system"),
		})
	}

	return rs
}

func newSublayerCalculator() *sublayerCalculator {
	return &sublayerCalculator{sc: stats.NewSublayerCalculator()}
}

func TestConvertToDatadogTd(t *testing.T) {
	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	calculator := newSublayerCalculator()

	outputTraces, err := convertToDatadogTd(traces, calculator, &config.Config{})

	assert.NoError(t, err)
	assert.Equal(t, 1, len(outputTraces))
}

func TestConvertToDatadogTdNoResourceSpans(t *testing.T) {
	traces := pdata.NewTraces()
	calculator := newSublayerCalculator()

	outputTraces, err := convertToDatadogTd(traces, calculator, &config.Config{})

	assert.NoError(t, err)
	assert.Equal(t, 0, len(outputTraces))
}

func TestObfuscation(t *testing.T) {
	calculator := newSublayerCalculator()

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	resource := rs.Resource()
	resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"service.name": pdata.NewAttributeValueString("sure"),
	})
	rs.InstrumentationLibrarySpans().Resize(1)
	ilss := rs.InstrumentationLibrarySpans().At(0)
	instrumentationLibrary := ilss.InstrumentationLibrary()
	instrumentationLibrary.SetName("flash")
	instrumentationLibrary.SetVersion("v1")

	ilss.Spans().Resize(1)
	span := ilss.Spans().At(0)

	// Make this a FaaS span, which will trigger an error, because conversion
	// of them is currently not supported.
	span.Attributes().InsertString("testinfo?=123", "http.route")

	outputTraces, err := convertToDatadogTd(traces, calculator, &config.Config{})

	assert.NoError(t, err)

	aggregatedTraces := aggregateTracePayloadsByEnv(outputTraces)

	obfuscator := obfuscate.NewObfuscator(obfuscatorConfig)

	obfuscatePayload(obfuscator, aggregatedTraces)

	assert.Equal(t, 1, len(aggregatedTraces))
}

func TestBasicTracesTranslation(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	// create mock resource span data
	// set shouldError and resourceServiceandEnv to false to test defaut behavior
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeUnset, false)

	// translate mocks to datadog traces
	datadogPayload, err := resourceSpansToDatadogSpans(rs, calculator, hostname, &config.Config{})

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure trace id gets translated to uint64 correctly
	assert.Equal(t, decodeAPMTraceID(mockTraceID), datadogPayload.Traces[0].TraceID)

	// ensure the correct number of spans are expected
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans))

	// ensure span's trace id matches payload trace id
	assert.Equal(t, datadogPayload.Traces[0].TraceID, datadogPayload.Traces[0].Spans[0].TraceID)

	// ensure span's spanId and parentSpanId are set correctly
	assert.Equal(t, decodeAPMSpanID(mockSpanID), datadogPayload.Traces[0].Spans[0].SpanID)
	assert.Equal(t, decodeAPMSpanID(mockParentSpanID), datadogPayload.Traces[0].Spans[0].ParentID)

	// ensure that span.resource defaults to otlp span.name
	assert.Equal(t, "End-To-End Here", datadogPayload.Traces[0].Spans[0].Resource)

	// ensure that span.name defaults to string representing instrumentation library if present
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", datadogPayload.Traces[0].Spans[0].Meta[tracetranslator.TagInstrumentationName], pdata.SpanKindSERVER)), datadogPayload.Traces[0].Spans[0].Name)

	// ensure that span.type is based on otlp span.kind
	assert.Equal(t, "web", datadogPayload.Traces[0].Spans[0].Type)

	// ensure that span.meta and span.metrics pick up attibutes, instrumentation ibrary and resource attribs
	assert.Equal(t, 8, len(datadogPayload.Traces[0].Spans[0].Meta))
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans[0].Metrics))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(0), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span meta also inccludes correctly sets resource attributes
	assert.Equal(t, "kube-system", datadogPayload.Traces[0].Spans[0].Meta["namespace"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "OTLPResourceNoServiceName", datadogPayload.Traces[0].Spans[0].Service)

	// ensure a duration and start time are calculated
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Start)
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Duration)
}

func TestTracesTranslationErrorsAndResource(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true)

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload, err := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(1), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span status code is set
	assert.Equal(t, "501", datadogPayload.Traces[0].Spans[0].Meta["http.status_code"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 14, len(datadogPayload.Traces[0].Spans[0].Meta))
}

func TestTracesTranslationOkStatus(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true)

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload, err := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(1), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span status code is set
	assert.Equal(t, "501", datadogPayload.Traces[0].Spans[0].Meta["http.status_code"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 14, len(datadogPayload.Traces[0].Spans[0].Meta))
}

// ensure that the datadog span uses the configured unified service tags
func TestTracesTranslationConfig(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeUnset, true)

	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "alt-service",
		},
	}

	// translate mocks to datadog traces
	datadogPayload, err := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status, unset should not error
	assert.Equal(t, int32(0), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 11, len(datadogPayload.Traces[0].Spans[0].Meta))
}

// ensure that the translation returns early if no resource instrumentation library spans
func TestTracesTranslationNoIls(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	rs := pdata.NewResourceSpans()

	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "alt-service",
		},
	}

	// translate mocks to datadog traces
	datadogPayload, err := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 0, len(datadogPayload.Traces))
}

// ensure that datadog span resource naming uses http method+route when available
func TestSpanResourceTranslation(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"http.method": "GET",
		"http.route":  "/api",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "GET /api", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that the datadog span name uses IL name +kind whenn available and falls back to opetelemetry + kind
func TestSpanNameTranslation(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("Default Name")
	span.SetKind(pdata.SpanKindSERVER)

	ddIlTags := map[string]string{
		fmt.Sprintf(tracetranslator.TagInstrumentationName): "il_name",
	}

	ddNoIlTags := map[string]string{
		"other": "other_value",
	}

	ddIlTagsOld := map[string]string{
		"otel.instrumentation_library.name": "old_value",
	}

	ddIlTagsCur := map[string]string{
		"otel.library.name": "current_value",
	}

	ddIlTagsUnusual := map[string]string{
		"otel.library.name": "@unusual/\\::value",
	}

	spanNameIl := getDatadogSpanName(span, ddIlTags)
	spanNameDefault := getDatadogSpanName(span, ddNoIlTags)
	spanNameOld := getDatadogSpanName(span, ddIlTagsOld)
	spanNameCur := getDatadogSpanName(span, ddIlTagsCur)
	spanNameUnusual := getDatadogSpanName(span, ddIlTagsUnusual)

	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "il_name", pdata.SpanKindSERVER)), spanNameIl)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "opentelemetry", pdata.SpanKindSERVER)), spanNameDefault)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "old_value", pdata.SpanKindSERVER)), spanNameOld)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "current_value", pdata.SpanKindSERVER)), spanNameCur)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "unusual_value", pdata.SpanKindSERVER)), spanNameUnusual)
}

// ensure that the datadog span type gets mapped from span kind
func TestSpanTypeTranslation(t *testing.T) {
	spanTypeClient := spanKindToDatadogType(pdata.SpanKindCLIENT)
	spanTypeServer := spanKindToDatadogType(pdata.SpanKindSERVER)
	spanTypeCustom := spanKindToDatadogType(pdata.SpanKindUNSPECIFIED)

	assert.Equal(t, "http", spanTypeClient)
	assert.Equal(t, "web", spanTypeServer)
	assert.Equal(t, "custom", spanTypeCustom)

}

// ensure that the IL Tags extraction handles nil case
func TestILTagsExctraction(t *testing.T) {
	il := pdata.NewInstrumentationLibrary()

	tags := map[string]string{}

	extractInstrumentationLibraryTags(il, tags)

	assert.Equal(t, "", tags[tracetranslator.TagInstrumentationName])

}

func TestHttpResourceTag(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("Default Name")
	span.SetKind(pdata.SpanKindSERVER)

	ddTags := map[string]string{
		"http.method": "POST",
	}

	resourceName := getDatadogResourceName(pdata.Span{}, ddTags)

	assert.Equal(t, "POST", resourceName)
}

// ensure that payloads get aggregated by env to reduce number of flushes
func TestTracePayloadAggr(t *testing.T) {

	// ensure that tracepayloads are aggregated by matchig hosts if envs match
	env := "testenv"
	payloadOne := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	payloadTwo := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	var originalPayload []*pb.TracePayload
	originalPayload = append(originalPayload, &payloadOne)
	originalPayload = append(originalPayload, &payloadTwo)

	updatedPayloads := aggregateTracePayloadsByEnv(originalPayload)

	assert.Equal(t, 2, len(originalPayload))
	assert.Equal(t, 1, len(updatedPayloads))

	// ensure that trace playloads are not aggregated by matching hosts if different envs
	envTwo := "testenvtwo"

	payloadThree := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	payloadFour := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          envTwo,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	var originalPayloadDifferentEnv []*pb.TracePayload
	originalPayloadDifferentEnv = append(originalPayloadDifferentEnv, &payloadThree)
	originalPayloadDifferentEnv = append(originalPayloadDifferentEnv, &payloadFour)

	updatedPayloadsDifferentEnv := aggregateTracePayloadsByEnv(originalPayloadDifferentEnv)

	assert.Equal(t, 2, len(originalPayloadDifferentEnv))
	assert.Equal(t, 2, len(updatedPayloadsDifferentEnv))
}
