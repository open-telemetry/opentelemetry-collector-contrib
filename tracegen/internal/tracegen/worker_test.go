// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracegen

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/api/global"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

func TestFixedNumberOfTraces(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	global.SetTracerProvider(tracerProvider)

	cfg := &Config{
		NumTraces:   1,
		WorkerCount: 1,
	}

	// test
	Run(cfg, zapr.NewLogger(zap.NewNop()))

	// verify
	assert.Len(t, syncer.spans, 2) // each trace has two spans
}

func TestRateOfSpans(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	global.SetTracerProvider(tracerProvider)

	cfg := &Config{
		Rate:          10,
		TotalDuration: time.Second / 2,
		WorkerCount:   1,
	}

	// sanity check
	require.Len(t, syncer.spans, 0)

	// test
	Run(cfg, zapr.NewLogger(zap.NewNop()))

	// verify
	// the minimum acceptable number of spans for the rate of 10/sec for half a second
	assert.True(t, len(syncer.spans) >= 6, "there should have been more than 6 spans, had %d", len(syncer.spans))
	// the maximum acceptable number of spans for the rate of 10/sec for half a second
	assert.True(t, len(syncer.spans) <= 20, "there should have been less than 20 spans, had %d", len(syncer.spans))
}

func TestUnthrottled(t *testing.T) {
	// prepare
	syncer := &mockSyncer{}

	tracerProvider := sdktrace.NewTracerProvider()
	sp := sdktrace.NewSimpleSpanProcessor(syncer)
	tracerProvider.RegisterSpanProcessor(sp)
	global.SetTracerProvider(tracerProvider)

	cfg := &Config{
		TotalDuration: 50 * time.Millisecond,
		WorkerCount:   1,
	}

	// sanity check
	require.Len(t, syncer.spans, 0)

	// test
	Run(cfg, zapr.NewLogger(zap.NewNop()))

	// verify
	// the minimum acceptable number of spans -- the real number should be > 10k, but CI env might be slower
	assert.True(t, len(syncer.spans) > 100, "there should have been more than 100 spans, had %d", len(syncer.spans))
}

var _ export.SpanExporter = (*mockSyncer)(nil)

type mockSyncer struct {
	spans []*export.SpanData
}

func (m *mockSyncer) ExportSpans(_ context.Context, spanData []*export.SpanData) error {
	m.spans = append(m.spans, spanData...)
	return nil
}

func (m *mockSyncer) Shutdown(context.Context) error {
	panic("implement me")
}

func (m *mockSyncer) Reset() {
	m.spans = []*export.SpanData{}
}
