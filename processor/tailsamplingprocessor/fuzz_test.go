// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func FuzzConsumeTraces(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &ptrace.JSONUnmarshaler{}
		traces, err := ju.UnmarshalTraces(data)
		if err != nil {
			return
		}
		sink := new(consumertest.TracesSink)
		set := processortest.NewNopSettings(metadata.Type)
		cfg := &Config{
			SamplingStrategy: samplingStrategyTraceComplete,
		}
		tsp, err := newTracesProcessor(t.Context(), set, sink, *cfg)
		if err != nil {
			t.Fatal(err)
		}
		_ = tsp.ConsumeTraces(t.Context(), traces)
	})
}

// FuzzSplitResourceSpansByTrace checks that the split that moves resources and
// scopes returns the same batches as the one that copies them.
func FuzzSplitResourceSpansByTrace(f *testing.F) {
	for _, in := range []splitInput{{{1, 2, -1}, {1}, {-2, -2}}, {{1}, {-1}}} {
		td := ptrace.NewTraces()
		in.build().CopyTo(td.ResourceSpans().AppendEmpty())
		seed, err := (&ptrace.JSONMarshaler{}).MarshalTraces(td)
		require.NoError(f, err)
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &ptrace.JSONUnmarshaler{}
		traces, err := ju.UnmarshalTraces(data)
		if err != nil {
			return
		}
		for _, rss := range traces.ResourceSpans().All() {
			requireSameSplit(t, rss)
		}
	})
}
