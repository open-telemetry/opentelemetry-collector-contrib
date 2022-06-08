// Copyright 2022 OpenTelemetry Authors
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
// See the License for the specific language

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/obsreport"
)

type MockTraceConsumer struct {
	verifier func(td ptrace.Traces) error
	err      error
}

func (mtc *MockTraceConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if mtc.verifier != nil {
		err := mtc.verifier(td)
		if err != nil {
			return err
		}
	}
	return mtc.err
}

func (mtc *MockTraceConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func TestOpenTelemetryTraceHandler(t *testing.T) {
    // Sample of raw opentelemtry msgpack encoded trace
	var data = []byte{156, 207, 184, 70, 109, 229, 195, 229, 42, 78, 207, 163, 89, 69, 137,
		44, 190, 108, 95, 207, 14, 54, 176, 196, 158, 40, 251, 244, 207, 0, 0, 0, 0, 0, 0, 0, 0,
		171, 84, 114, 97, 110, 115, 97, 99, 116, 105, 111, 110, 203, 65, 216, 167, 126, 179, 33,
		156, 246, 203, 65, 216, 167, 126, 179, 33, 156, 246, 204, 2, 204, 1, 144, 144, 129, 167,
		97, 100, 100, 114, 101, 115, 115, 174, 49, 50, 55, 46, 48, 46, 48, 46, 49, 58, 52, 55, 48,
		48}

	verifyTrace := func(td ptrace.Traces) error {
		assert.Equal(t, 1, td.SpanCount())
		spans := td.ResourceSpans()
		span := spans.At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "4e2ae5c3e56d46b85f6cbe2c894559a3", span.TraceID().HexString())
		assert.Equal(t, "f4fb289ec4b0360e", span.SpanID().HexString())
		assert.Equal(t, "2022-06-06 13:02:04.525205135 +0000 UTC", span.StartTimestamp().String())
		assert.Equal(t, "2022-06-06 13:02:04.525205135 +0000 UTC", span.EndTimestamp().String())
		assert.Equal(t, "", span.ParentSpanID().HexString())
		assert.Equal(t, "Transaction", span.Name())
		assert.Equal(t, "SPAN_KIND_SERVER", span.Kind().String())
		assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
		assert.Equal(t, uint32(0), span.DroppedEventsCount())
		assert.Equal(t, uint32(0), span.DroppedAttributesCount())
		assert.Equal(t, uint32(0), span.DroppedLinksCount())
		assert.Equal(t, 0, span.Links().Len())
		assert.Equal(t, 0, span.Events().Len())
		assert.Equal(t, 1, span.Attributes().Len())
		attr, ok := span.Attributes().Get("address")
		assert.True(t, ok)
		assert.Equal(t, "127.0.0.1:4700", attr.AsString())
		return nil
	}

	mockConsumer := MockTraceConsumer{verifier: verifyTrace}
	config := createDefaultConfig()
	settings := componenttest.NewNopReceiverCreateSettings()
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})

	handler := openTelemetryHandler{consumer: &mockConsumer, obsrecv: obsrecv}
	err := handler.Handle(data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenTelemetryMalformed(t *testing.T) {
	config := createDefaultConfig()
	settings := componenttest.NewNopReceiverCreateSettings()
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})

	handler := openTelemetryHandler{obsrecv: obsrecv}
	err := handler.Handle([]byte("foo"))
	assert.Error(t, err, "expected error")
}

func TestOpenTracingHandler(t *testing.T) {
	trace := &OpenTracing{
		ArrLen:         1,
		SourceIP:       "192.158.0.1:4000",
		TraceID:        8793247892340890,
		SpanID:         2389203490823490,
		StartTimestamp: 1646334304.666,
		Duration:       1000,
		OperationName:  "StorageUpdate",
		Tags:           map[string]interface{}{},
		ParentSpanIDs:  []interface{}{90823908902384},
	}
	data, err := msgpack.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}

	verifyTrace := func(td ptrace.Traces) error {
		assert.Equal(t, 1, td.SpanCount())
		spans := td.ResourceSpans()
		span := spans.At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "00000000000000009a800b91693d1f00", span.TraceID().HexString())
		assert.Equal(t, "42dd5dc9f77c0800", span.SpanID().HexString())
		assert.Equal(t, "2022-03-03 19:05:04.665999889 +0000 UTC", span.StartTimestamp().String())
		assert.Equal(t, "2022-03-03 19:21:44.665999889 +0000 UTC", span.EndTimestamp().String())
		assert.Equal(t, ptrace.SpanKind(2), span.Kind())
		assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
		assert.Equal(t, "StorageUpdate", span.Name())
		assert.Equal(t, uint32(0), span.DroppedEventsCount())
		assert.Equal(t, uint32(0), span.DroppedAttributesCount())
		assert.Equal(t, uint32(0), span.DroppedLinksCount())
		assert.Equal(t, "f0c5d3969a520000", span.ParentSpanID().HexString())
		attr, ok := span.Attributes().Get("sourceIP")
		assert.True(t, ok)
		assert.Equal(t, "192.158.0.1:4000", attr.StringVal())
		return nil
	}

	mockConsumer := MockTraceConsumer{verifier: verifyTrace}
	config := createDefaultConfig()
	settings := componenttest.NewNopReceiverCreateSettings()
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})

	handler := openTracingHandler{consumer: &mockConsumer, obsrecv: obsrecv}
	err = handler.Handle(data)
	assert.NoError(t, err)
}

func TestOpenTracingMalformed(t *testing.T) {
	config := createDefaultConfig()
	settings := componenttest.NewNopReceiverCreateSettings()
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})

	handler := openTracingHandler{obsrecv: obsrecv}
	err := handler.Handle([]byte("foo"))
	assert.Error(t, err, "expected error")
}

func BenchmarkOpenTracingNoTags(b *testing.B) {
	trace := &OpenTracing{
		ArrLen:         1,
		SourceIP:       "192.158.0.1:4000",
		TraceID:        8793247892340890,
		SpanID:         2389203490823490,
		StartTimestamp: 12282389238923,
		Duration:       100,
		OperationName:  "foobar",
		Tags:           map[string]interface{}{},
		ParentSpanIDs:  []interface{}{90823908902384},
	}
	data, err := msgpack.Marshal(trace)
	if err != nil {
		b.Fatal(err)
	}

	handler := openTracingHandler{consumer: &MockTraceConsumer{}, obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{})}
	for i := 0; i < b.N; i++ {
		err := handler.Handle(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOpenTracingTagsAndParents(b *testing.B) {
	trace := &OpenTracing{
		ArrLen:         1,
		SourceIP:       "192.158.0.1:4000",
		TraceID:        8793247892340890,
		SpanID:         2389203490823490,
		StartTimestamp: 12282389238923,
		Duration:       100,
		OperationName:  "foobar",
		Tags: map[string]interface{}{
			"foo":               "a very long value should go here",
			"customerID":        "abc-555444-asx",
			"jobID":             "78989234920-234-0234908",
			"availability-zone": "us-west-2c",
			"region":            "us-west",
		},
		ParentSpanIDs: []interface{}{90823908902384, 989789796876868},
	}
	data, err := msgpack.Marshal(trace)
	if err != nil {
		b.Fatal(err)
	}

	handler := openTracingHandler{consumer: &MockTraceConsumer{}, obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{})}
	for i := 0; i < b.N; i++ {
		err := handler.Handle(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
