// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

const (
	telemetryAttrKeyOne   = "k1"
	telemetryAttrKeyTwo   = "k2"
	telemetryAttrValueOne = "v1"
	telemetryAttrValueTwo = "v2"
)

func TestFixedNumberOfTraces(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumTraces: 1,
	}

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	assert.Len(t, syncer.spans, 2) // each trace has two spans
}

func TestRateOfSpans(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
	}

	// sanity check
	require.Len(t, syncer.spans, 0)

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	// the minimum acceptable number of spans for the rate of 10/sec for half a second
	assert.True(t, len(syncer.spans) >= 6, "there should have been more than 6 spans, had %d", len(syncer.spans))
	// the maximum acceptable number of spans for the rate of 10/sec for half a second
	assert.True(t, len(syncer.spans) <= 20, "there should have been less than 20 spans, had %d", len(syncer.spans))
}

func TestSpanDuration(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	targetDuration := 1 * time.Second
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
		SpanDuration: targetDuration,
	}

	// sanity check
	require.Len(t, syncer.spans, 0)

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	for _, span := range syncer.spans {
		startTime, endTime := span.StartTime(), span.EndTime()
		spanDuration := endTime.Sub(startTime)
		assert.Equal(t, targetDuration, spanDuration)
	}
}

func TestUnthrottled(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := &Config{
		Config: common.Config{
			TotalDuration: 50 * time.Millisecond,
			WorkerCount:   1,
		},
	}

	// sanity check
	require.Len(t, syncer.spans, 0)

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	// the minimum acceptable number of spans -- the real number should be > 10k, but CI env might be slower
	assert.True(t, len(syncer.spans) > 100, "there should have been more than 100 spans, had %d", len(syncer.spans))
}

func TestSpanKind(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumTraces: 1,
	}

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify that the default Span Kind is being overridden
	for _, span := range syncer.spans {
		assert.NotEqual(t, span.SpanKind(), trace.SpanKindInternal)
	}
}

func TestSpanStatuses(t *testing.T) {
	tests := []struct {
		inputStatus string
		spanStatus  codes.Code
		validInput  bool
	}{
		{inputStatus: `Unset`, spanStatus: codes.Unset, validInput: true},
		{inputStatus: `Error`, spanStatus: codes.Error, validInput: true},
		{inputStatus: `Ok`, spanStatus: codes.Ok, validInput: true},
		{inputStatus: `unset`, spanStatus: codes.Unset, validInput: true},
		{inputStatus: `error`, spanStatus: codes.Error, validInput: true},
		{inputStatus: `ok`, spanStatus: codes.Ok, validInput: true},
		{inputStatus: `UNSET`, spanStatus: codes.Unset, validInput: true},
		{inputStatus: `ERROR`, spanStatus: codes.Error, validInput: true},
		{inputStatus: `OK`, spanStatus: codes.Ok, validInput: true},
		{inputStatus: `0`, spanStatus: codes.Unset, validInput: true},
		{inputStatus: `1`, spanStatus: codes.Error, validInput: true},
		{inputStatus: `2`, spanStatus: codes.Ok, validInput: true},
		{inputStatus: `Foo`, spanStatus: codes.Unset, validInput: false},
		{inputStatus: `-1`, spanStatus: codes.Unset, validInput: false},
		{inputStatus: `3`, spanStatus: codes.Unset, validInput: false},
		{inputStatus: `Err`, spanStatus: codes.Unset, validInput: false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("inputStatus=%s", tt.inputStatus), func(t *testing.T) {
			syncer := &mockSyncer{}

			tracerProvider := sdktrace.NewTracerProvider()
			sp := sdktrace.NewSimpleSpanProcessor(syncer)
			tracerProvider.RegisterSpanProcessor(sp)
			otel.SetTracerProvider(tracerProvider)

			cfg := &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumTraces:  1,
				StatusCode: tt.inputStatus,
			}

			// test the program given input, including erroneous inputs
			if tt.validInput {
				require.NoError(t, Run(cfg, zap.NewNop()))
				// verify that the default the span status is set as expected
				for _, span := range syncer.spans {
					assert.Equal(t, span.Status().Code, tt.spanStatus, fmt.Sprintf("span status: %v and expected status %v", span.Status().Code, tt.spanStatus))
				}
			} else {
				require.Error(t, Run(cfg, zap.NewNop()))
			}
		})
	}
}

func TestSpansWithNoAttrs(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := configWithNoAttributes(2, "")

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	assert.Len(t, syncer.spans, 4) // each trace has two spans
	for _, span := range syncer.spans {
		attributes := span.Attributes()
		assert.Equal(t, 2, len(attributes), "it shouldn't have more than 2 fixed attributes")
	}
}

func TestSpansWithOneAttrs(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := configWithOneAttribute(2, "")

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	assert.Len(t, syncer.spans, 4) // each trace has two spans
	for _, span := range syncer.spans {
		attributes := span.Attributes()
		assert.Equal(t, 3, len(attributes), "it should have more than 3 attributes")
	}
}

func TestSpansWithMultipleAttrs(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	otel.SetTracerProvider(tracerProvider)

	cfg := configWithMultipleAttributes(2, "")

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	assert.Len(t, syncer.spans, 4) // each trace has two spans
	for _, span := range syncer.spans {
		attributes := span.Attributes()
		assert.Equal(t, 4, len(attributes), "it should have more than 4 attributes")
	}
}

var _ sdktrace.SpanExporter = (*mockSyncer)(nil)

type mockSyncer struct {
	spans []sdktrace.ReadOnlySpan
}

func (m *mockSyncer) ExportSpans(_ context.Context, spanData []sdktrace.ReadOnlySpan) error {
	m.spans = append(m.spans, spanData...)
	return nil
}

func (m *mockSyncer) Shutdown(context.Context) error {
	panic("implement me")
}

func (m *mockSyncer) Reset() {
	m.spans = []sdktrace.ReadOnlySpan{}
}

func configWithNoAttributes(qty int, statusCode string) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
		},
		NumTraces:  qty,
		StatusCode: statusCode,
	}
}

func configWithOneAttribute(qty int, statusCode string) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
		},
		NumTraces:  qty,
		StatusCode: statusCode,
	}
}

func configWithMultipleAttributes(qty int, statusCode string) *Config {
	kvs := common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne, telemetryAttrKeyTwo: telemetryAttrValueTwo}
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
		},
		NumTraces:  qty,
		StatusCode: statusCode,
	}

}
