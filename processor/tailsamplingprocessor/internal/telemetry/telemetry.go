// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"

import (
	"context"
	"sync"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/config/configtelemetry"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

var onceTelemetry sync.Once

type T struct {
	ContextForPolicy      func(ctx context.Context, configName, format string) (context.Context, error)
	RecordPolicyLatency   func(ctx context.Context, latencyMicroSec int64)
	RecordPolicyDecision  func(ctx context.Context, sampled bool, numSpans int64)
	RecordNewTraceIDs     func(ctx context.Context, count int64)
	RecordLateSpan        func(ctx context.Context, ageSec int64)
	RecordTraceRemovalAge func(ctx context.Context, ageSec int64)
	RecordFinalDecision   func(ctx context.Context, latencyMicroSec, droppedTooEarly, evaluationErrors, tracesOnMemory int64, decision sampling.Decision)
}

func New() *T {
	onceTelemetry.Do(func() {
		_ = view.Register(samplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return &T{
		ContextForPolicy:      contextForPolicyOC,
		RecordPolicyLatency:   recordPolicyLatencyOC,
		RecordPolicyDecision:  recordPolicyDecisionOC,
		RecordNewTraceIDs:     recordNewTraceIDsOC,
		RecordLateSpan:        recordLateSpanOC,
		RecordTraceRemovalAge: recordTraceRemovalAgeOC,
		RecordFinalDecision:   recordFinalDecisionOC,
	}
}
