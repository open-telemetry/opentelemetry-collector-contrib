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

package awsxrayexporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry/telemetrytest"
)

func TestTraceExport(t *testing.T) {
	traceExporter := initializeTracesExporter(t, generateConfig(t), telemetrytest.NewNopRegistry())
	ctx := context.Background()
	td := constructSpanData()
	err := traceExporter.ConsumeTraces(ctx, td)
	assert.NotNil(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.Nil(t, err)
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
	assert.NotNil(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.Nil(t, err)
}

func TestXrayAndW3CSpanTraceResourceExtraction(t *testing.T) {
	td := constructXrayAndW3CSpanData()
	logger, _ := zap.NewProduction()
	assert.Len(t, extractResourceSpans(generateConfig(t), logger, td), 2, "2 spans have xay trace id")
}

func TestW3CSpanTraceResourceExtraction(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9255")
	td := constructW3CSpanData()
	logger, _ := zap.NewProduction()
	assert.Len(t, extractResourceSpans(generateConfig(t), logger, td), 0, "0 spans have xray trace id")
}

func TestTelemetryEnabled(t *testing.T) {
	// replace global registry for test
	registry := telemetry.NewRegistry()
	sink := telemetrytest.NewSenderSink()
	// preload the sender that the exporter will use
	sender, loaded := registry.LoadOrStore(component.NewID(""), sink)
	require.False(t, loaded)
	require.NotNil(t, sender)
	require.Equal(t, sink, sender)
	cfg := generateConfig(t)
	cfg.TelemetryConfig.Enabled = true
	traceExporter := initializeTracesExporter(t, cfg, registry)
	ctx := context.Background()
	assert.NoError(t, traceExporter.Start(ctx, componenttest.NewNopHost()))
	td := constructSpanData()
	err := traceExporter.ConsumeTraces(ctx, td)
	assert.NotNil(t, err)
	err = traceExporter.Shutdown(ctx)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, sink.StartCount.Load())
	assert.EqualValues(t, 1, sink.StopCount.Load())
	assert.True(t, sink.HasRecording())
	got := sink.Rotate()
	assert.EqualValues(t, 1, *got.BackendConnectionErrors.HTTPCode4XXCount)
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

func initializeTracesExporter(t testing.TB, exporterConfig *Config, registry telemetry.Registry) exporter.Traces {
	t.Helper()
	mconn := new(awsutil.Conn)
	traceExporter, err := newTracesExporter(exporterConfig, exportertest.NewNopCreateSettings(), mconn, registry)
	if err != nil {
		panic(err)
	}
	return traceExporter
}

func generateConfig(t testing.TB) *Config {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIASSWVJUY4PZXXXXXX")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "XYrudg2H87u+ADAAq19Wqx3D41a09RsTXXXXXXXX")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	t.Setenv("AWS_REGION", "us-east-1")
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

// nolint:unused
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
	attrs.PutStr(conventions.AttributeServiceName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeContainerName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventions.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudAccountID, "999999998")
	attrs.PutStr(conventions.AttributeCloudRegion, "us-west-2")
	attrs.PutStr(conventions.AttributeCloudAvailabilityZone, "us-west-1b")
	return resource
}

func constructHTTPClientSpan(traceID pcommon.TraceID) ptrace.Span {
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPStatusCode] = 200
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
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventions.AttributeHTTPStatusCode] = 200
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

func constructSpanAttributes(attributes map[string]interface{}) pcommon.Map {
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
	for i := range r {
		r[i] = byte(rand.Intn(128))
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
