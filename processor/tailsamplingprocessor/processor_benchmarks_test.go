// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func BenchmarkSampling(b *testing.B) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(context.Background(), processortest.NewNopSettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	require.NoError(b, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, tsp.Shutdown(context.Background()))
	}()
	metrics := &policyMetrics{}
	sampleBatches := make([]*sampling.TraceData, 0, len(batches))

	for i := 0; i < len(batches); i++ {
		sampleBatches = append(sampleBatches, &sampling.TraceData{
			ArrivalTime: time.Now(),
			// SpanCount:       spanCount,
			ReceivedBatches: ptrace.NewTraces(),
		})
	}

	for i := 0; i < b.N; i++ {
		for i, id := range traceIDs {
			_ = tsp.makeDecision(id, sampleBatches[i], metrics)
		}
	}
}
