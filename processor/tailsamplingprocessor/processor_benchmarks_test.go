// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
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
	sp, _ := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	require.NoError(b, tsp.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, tsp.Shutdown(b.Context()))
	}()
	metrics := &policyMetrics{}

	for i, id := range traceIDs {
		sb := &sampling.TraceData{
			ArrivalTime:     time.Now(),
			ReceivedBatches: batches[i],
		}
		_ = tsp.makeDecision(id, sb, metrics)
	}
}
