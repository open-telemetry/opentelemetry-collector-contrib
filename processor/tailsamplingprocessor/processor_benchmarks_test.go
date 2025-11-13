// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func BenchmarkSampling(b *testing.B) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	require.NoError(b, tsp.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, tsp.Shutdown(b.Context()))
	}()
	metrics := newPolicyMetrics(len(cfg.PolicyCfgs))
	sampleBatches := make([]*samplingpolicy.TraceData, 0, len(batches))

	for _, batch := range batches {
		spanCount := &atomic.Int64{}
		spanCount.Store(int64(batch.SpanCount()))
		sampleBatches = append(sampleBatches, &samplingpolicy.TraceData{
			ArrivalTime:     time.Now(),
			SpanCount:       spanCount,
			ReceivedBatches: batch,
		})
	}

	for b.Loop() {
		for i, id := range traceIDs {
			_ = tsp.makeDecision(id, sampleBatches[i], metrics)
		}
	}
}
