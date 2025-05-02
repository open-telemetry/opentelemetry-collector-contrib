// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv112 "go.opentelemetry.io/collector/semconv/v1.12.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry/telemetrytest"
)

func TestTraceExport(t *testing.T) {
	traceExporter := initializeTracesExporter(t, generateConfig(t), telemetrytest.NewNopRegistry())
	ctx := context.Background()
	td := constructSpanData()
	err := traceExporter.ConsumeTraces(ctx, td)
	assert.Error(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestXraySpanTraceResourceExtraction(t *testing.T) {
	td := constructSpanData()
	logger, _ := zap.NewProduction()
	assert.Len(t, extractResourceSpans(generateConfig(t), logger, td), 2, "2 spans have xay trace id")
}

func TestXrayAndW3CSpanTraceExport(t *testing.T) {
	traceExporter := initializeTracesExporter(t, generateConfig(t), telemetrytest.NewNopRegistry())
	ctx := context.Background()
	td := constructXrayAndW3CSpanData()
	err := traceExporter.ConsumeTraces(ctx, td)
	assert.Error(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestXrayAndW3CSpanTraceResourceExtraction(t *testing.T) {
	td := constructXrayAndW3CSpanData()
	logger, _ := zap.NewProduction()
	assert.Len(t, extractResourceSpans(generateConfig(t), logger, td), 4, "4 spans have xray/w3c trace id")
}

func TestW3CSpanTraceResourceExtraction(t *testing.T) {
	td := constructW3CSpanData()
	logger, _ := zap.NewProduction()
	assert.Len(t, extractResourceSpans(generateConfig(t), logger, td), 2, "2 spans have w3c trace id")
}

func TestTelemetryEnabled(t *testing.T) {
	// replace global registry for test
	registry := telemetry.NewRegistry()
	sink := telemetrytest.NewSenderSink()
	// preload the sender that the exporter will use
	set := exportertest.NewNopSettings(metadata.Type)
	sender, loaded := registry.LoadOrStore(set.ID, sink)
	require.False(t, loaded)
	require.NotNil(t, sender)
	require.Equal(t, sink, sender)
	cfg := generateConfig(t)
	cfg.TelemetryConfig.Enabled = true
	traceExporter, err := newTracesExporter(context.Background(), cfg, set, registry)
	assert.NoError(t, err)
	ctx := context.Background()
	assert.NoError(t, traceExporter.Start(ctx, componenttest.NewNopHost()))
	td := constructSpanData()
	err = traceExporter.ConsumeTraces(ctx, td)
	assert.Error(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, sink.StartCount.Load())
	assert.EqualValues(t, 1, sink.StopCount.Load())
	assert.True(t, sink.HasRecording())
	got := sink.Rotate()
	assert.EqualValues(t, 0, *got.BackendConnectionErrors.HTTPCode4XXCount)
}

func BenchmarkForTracesExporter(b *testing.B) {
	traceExporter := initializeTracesExporter(b, generateConfig(b), telemetrytest.NewNopRegistry())
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ctx := context.Background()
		td := constructSpanData()
		b.StartTimer()
		err := traceExporter.ConsumeTraces(ctx, td)
		assert.Error(b, err)
	}
}

func initializeTracesExporter(tb testing.TB, exporterConfig *Config, registry telemetry.Registry) exporter.Traces {
	tb.Helper()
	traceExporter, err := newTracesExporter(context.Background(), exporterConfig, exportertest.NewNopSettings(metadata.Type), registry)
	if err != nil {
		panic(err)
	}
	return traceExporter
}

func generateConfig(tb testing.TB) *Config {
	tb.Setenv("AWS_ACCESS_KEY_ID", "AKIASSWVJUY4PZXXXXXX")
	tb.Setenv("AWS_SECRET_ACCESS_KEY", "XYrudg2H87u+ADAAq19Wqx3D41a09RsTXXXXXXXX")
	tb.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	tb.Setenv("AWS_REGION", "us-east-1")
	factory := NewFactory()
	exporterConfig := factory.CreateDefaultConfig().(*Config)
	exporterConfig.Region = "us-east-1"
	exporterConfig.LocalMode = true
	return exporterConfig
}

func constructSpanData() ptrace.Traces {
	resource := constructResource()

	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	resource.CopyTo(rspans.Resource())
	ispans := rspans.ScopeSpans().AppendEmpty()
	constructXrayTraceSpanData(ispans)
	return traces
}

func constructW3CSpanData() ptrace.Traces {
	resource := constructResource()
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	resource.CopyTo(rspans.Resource())
	ispans := rspans.ScopeSpans().AppendEmpty()
	constructW3CFormatTraceSpanData(ispans)
	return traces
}

func constructXrayAndW3CSpanData() ptrace.Traces {
	resource := constructResource()
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	resource.CopyTo(rspans.Resource())
	ispans := rspans.ScopeSpans().AppendEmpty()
	constructXrayTraceSpanData(ispans)
	constructW3CFormatTraceSpanData(ispans)
	return traces
}

func constructXrayTraceSpanData(ispans ptrace.ScopeSpans) {
	constructHTTPClientSpan(newTraceID()).CopyTo(ispans.Spans().AppendEmpty())
	constructHTTPServerSpan(newTraceID()).CopyTo(ispans.Spans().AppendEmpty())
}

func constructW3CFormatTraceSpanData(ispans ptrace.ScopeSpans) {
	constructHTTPClientSpan(constructW3CTraceID()).CopyTo(ispans.Spans().AppendEmpty())
	constructHTTPServerSpan(constructW3CTraceID()).CopyTo(ispans.Spans().AppendEmpty())
}

func constructResource() pcommon.Resource {
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventionsv112.AttributeServiceName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeContainerName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "999999998")
	attrs.PutStr(conventionsv112.AttributeCloudRegion, "us-west-2")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-west-1b")
	return resource
}

func constructHTTPClientSpan(traceID pcommon.TraceID) ptrace.Span {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructHTTPServerSpan(traceID pcommon.TraceID) ptrace.Span {
	attributes := make(map[string]any)
	attributes[conventionsv112.AttributeHTTPMethod] = http.MethodGet
	attributes[conventionsv112.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventionsv112.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventionsv112.AttributeHTTPStatusCode] = 200
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func constructSpanAttributes(attributes map[string]any) pcommon.Map {
	attrs := pcommon.NewMap()
	for key, value := range attributes {
		if cast, ok := value.(int); ok {
			attrs.PutInt(key, int64(cast))
		} else if cast, ok := value.(int64); ok {
			attrs.PutInt(key, cast)
		} else {
			attrs.PutStr(key, fmt.Sprintf("%v", value))
		}
	}
	return attrs
}

func newTraceID() pcommon.TraceID {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return r
}

func constructW3CTraceID() pcommon.TraceID {
	var r [16]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return r
}

func newSegmentID() pcommon.SpanID {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return r
}
