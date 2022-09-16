// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanbatchfilterprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestSpanBatchFilterProcessorSpansDelivered(t *testing.T) {
	sink := new(consumertest.TracesSink)
	cfg := createDefaultConfig().(*Config)
	cfg.TokensPerBatch = 1000
	creationSet := componenttest.NewNopProcessorCreateSettings()
	batcher, err := newBatchFilterProcessor(creationSet, sink, cfg, configtelemetry.LevelDetailed)
	require.NoError(t, err)
	require.NoError(t, batcher.Start(context.Background(), componenttest.NewNopHost()))

	requestCount := 1000
	spansPerRequest := 100
	traceDataSlice := make([]ptrace.Traces, 0, requestCount)
	for requestNum := 0; requestNum < requestCount; requestNum++ {
		td := testdata.GenerateTracesManySpansSameResource(spansPerRequest)
		spans := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
		for spanIndex := 0; spanIndex < spansPerRequest; spanIndex++ {
			spans.At(spanIndex).SetName(getTestSpanName(requestNum, spanIndex))
		}
		traceDataSlice = append(traceDataSlice, td.Clone())
		assert.NoError(t, batcher.ConsumeTraces(context.Background(), td))
	}

	// check for empty resources.
	td := ptrace.NewTraces()
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td))

	require.NoError(t, batcher.Shutdown(context.Background()))

	require.Equal(t, requestCount*spansPerRequest, sink.SpanCount())
	receivedTraces := sink.AllTraces()
	spansReceivedByName := spansReceivedByName(receivedTraces)
	for requestNum := 0; requestNum < requestCount; requestNum++ {
		spans := traceDataSlice[requestNum].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
		for spanIndex := 0; spanIndex < spansPerRequest; spanIndex++ {
			require.EqualValues(t,
				spans.At(spanIndex),
				spansReceivedByName[getTestSpanName(requestNum, spanIndex)])
		}
	}
}

func TestSpanBatchFilterProcessorSpansDeliveredExcessTokens(t *testing.T) {
	sink := new(consumertest.TracesSink)
	cfg := createDefaultConfig().(*Config)
	cfg.TokensPerBatch = 1000 // more tokens then spans
	creationSet := componenttest.NewNopProcessorCreateSettings()
	batcher, err := newBatchFilterProcessor(creationSet, sink, cfg, configtelemetry.LevelDetailed)
	require.NoError(t, err)
	require.NoError(t, batcher.Start(context.Background(), componenttest.NewNopHost()))

	requestCount := 100
	td := testdata.GenerateTracesManySpansSameResource(requestCount)
	spans := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	for requestNum := 0; requestNum < requestCount; requestNum++ {
		spans.At(0).SetName(getTestSpanName(requestNum, requestNum))
	}
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td))

	td2 := ptrace.NewTraces()
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td2))

	require.NoError(t, batcher.Shutdown(context.Background()))

	require.Equal(t, requestCount, sink.SpanCount())
}

func TestSpanBatchFilterProcessorSpansDeliveredExcessSpans(t *testing.T) {
	sink := new(consumertest.TracesSink)
	cfg := createDefaultConfig().(*Config)
	cfg.TokensPerBatch = 100 // less tokens then spans
	creationSet := componenttest.NewNopProcessorCreateSettings()
	batcher, err := newBatchFilterProcessor(creationSet, sink, cfg, configtelemetry.LevelDetailed)
	require.NoError(t, err)
	require.NoError(t, batcher.Start(context.Background(), componenttest.NewNopHost()))

	requestCount := 1000
	td := testdata.GenerateTracesManySpansSameResource(requestCount)
	spans := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	for requestNum := 0; requestNum < requestCount; requestNum++ {
		spans.At(0).SetName(getTestSpanName(requestNum, requestNum))
	}
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td))

	td2 := ptrace.NewTraces()
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td2))

	require.NoError(t, batcher.Shutdown(context.Background()))

	require.Equal(t, cfg.TokensPerBatch, sink.SpanCount())
}

func TestSpanBatchFilterProcessorTraceSendWhenClosing(t *testing.T) {
	cfg := Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		TokensPerBatch:    100,
	}
	sink := new(consumertest.TracesSink)

	creationSet := componenttest.NewNopProcessorCreateSettings()
	batcher, err := newBatchFilterProcessor(creationSet, sink, &cfg, configtelemetry.LevelDetailed)
	require.NoError(t, err)
	require.NoError(t, batcher.Start(context.Background(), componenttest.NewNopHost()))
	spans := 10

	td := testdata.GenerateTracesManySpansSameResource(spans)
	assert.NoError(t, batcher.ConsumeTraces(context.Background(), td))

	require.NoError(t, batcher.Shutdown(context.Background()))
	require.Equal(t, spans, sink.SpanCount())
	require.Equal(t, 1, len(sink.AllTraces()))
}

func TestShutdown(t *testing.T) {
	factory := NewFactory()
	componenttest.VerifyProcessorShutdown(t, factory, factory.CreateDefaultConfig())
}

func getTestSpanName(requestNum, index int) string {
	return fmt.Sprintf("test-span-%d-%d", requestNum, index)
}

func spansReceivedByName(tds []ptrace.Traces) map[string]ptrace.Span {
	spansReceivedByName := map[string]ptrace.Span{}
	for i := range tds {
		rss := tds[i].ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ilss := rss.At(i).ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)
					spansReceivedByName[spans.At(k).Name()] = span
				}
			}
		}
	}
	return spansReceivedByName
}
