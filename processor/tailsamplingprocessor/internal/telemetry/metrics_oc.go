// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// Variables related to metrics specific to tail sampling.
var (
	tagPolicyKey, _    = tag.NewKey("policy")
	tagSampledKey, _   = tag.NewKey("sampled")
	tagSourceFormat, _ = tag.NewKey("source_format")

	tagMutatorSampled    = []tag.Mutator{tag.Upsert(tagSampledKey, "true")}
	tagMutatorNotSampled = []tag.Mutator{tag.Upsert(tagSampledKey, "false")}

	statDecisionLatencyMicroSec  = stats.Int64("sampling_decision_latency", "Latency (in microseconds) of a given sampling policy", "µs")
	statOverallDecisionLatencyUs = stats.Int64("sampling_decision_timer_latency", "Latency (in microseconds) of each run of the sampling decision timer", "µs")

	statTraceRemovalAgeSec           = stats.Int64("sampling_trace_removal_age", "Time (in seconds) from arrival of a new trace until its removal from memory", "s")
	statLateSpanArrivalAfterDecision = stats.Int64("sampling_late_span_age", "Time (in seconds) from the sampling decision was taken and the arrival of a late span", "s")

	statPolicyEvaluationErrorCount = stats.Int64("sampling_policy_evaluation_error", "Count of sampling policy evaluation errors", stats.UnitDimensionless)

	statCountTracesSampled       = stats.Int64("count_traces_sampled", "Count of traces that were sampled or not per sampling policy", stats.UnitDimensionless)
	statCountSpansSampled        = stats.Int64("count_spans_sampled", "Count of spans that were sampled or not per sampling policy", stats.UnitDimensionless)
	statCountGlobalTracesSampled = stats.Int64("global_count_traces_sampled", "Global count of traces that were sampled or not by at least one policy", stats.UnitDimensionless)

	statDroppedTooEarlyCount    = stats.Int64("sampling_trace_dropped_too_early", "Count of traces that needed to be dropped before the configured wait time", stats.UnitDimensionless)
	statNewTraceIDReceivedCount = stats.Int64("new_trace_id_received", "Counts the arrival of new traces", stats.UnitDimensionless)
	statTracesOnMemoryGauge     = stats.Int64("sampling_traces_on_memory", "Tracks the number of traces current on memory", stats.UnitDimensionless)
)

func contextForPolicyOC(ctx context.Context, configName, format string) (context.Context, error) {
	return tag.New(ctx, tag.Upsert(tagPolicyKey, configName), tag.Upsert(tagSourceFormat, format))
}

func recordFinalDecisionOC(ctx context.Context, latencyMicroSec, droppedTooEarly, evaluationErrors, tracesOnMemory int64, decision sampling.Decision) {
	stats.Record(ctx,
		statOverallDecisionLatencyUs.M(latencyMicroSec),
		statDroppedTooEarlyCount.M(droppedTooEarly),
		statPolicyEvaluationErrorCount.M(evaluationErrors),
		statTracesOnMemoryGauge.M(tracesOnMemory),
	)

	var mutators []tag.Mutator
	switch decision {
	case sampling.Sampled:
		mutators = tagMutatorSampled
	case sampling.NotSampled:
		mutators = tagMutatorNotSampled
	}

	_ = stats.RecordWithTags(
		ctx,
		mutators,
		statCountGlobalTracesSampled.M(int64(1)),
	)
}

func recordPolicyLatencyOC(ctx context.Context, latencyMicroSec int64) {
	stats.Record(ctx,
		statDecisionLatencyMicroSec.M(latencyMicroSec),
	)
}

func recordPolicyDecisionOC(ctx context.Context, sampled bool, numSpans int64) {
	var mutators []tag.Mutator
	if sampled {
		mutators = tagMutatorSampled
	} else {
		mutators = tagMutatorNotSampled
	}

	_ = stats.RecordWithTags(
		ctx,
		mutators,
		statCountTracesSampled.M(int64(1)),
	)
	if isMetricStatCountSpansSampledEnabled() {
		_ = stats.RecordWithTags(
			ctx,
			mutators,
			statCountSpansSampled.M(numSpans),
		)
	}

}

func recordNewTraceIDsOC(ctx context.Context, count int64) {
	stats.Record(ctx, statNewTraceIDReceivedCount.M(count))
}

func recordLateSpanOC(ctx context.Context, ageSec int64) {
	stats.Record(ctx, statLateSpanArrivalAfterDecision.M(ageSec))
}

func recordTraceRemovalAgeOC(ctx context.Context, ageSec int64) {
	stats.Record(ctx, statTraceRemovalAgeSec.M(ageSec))
}

// samplingProcessorMetricViews return the metrics views according to given telemetry level.
func samplingProcessorMetricViews(level configtelemetry.Level) []*view.View {
	if level == configtelemetry.LevelNone {
		return nil
	}

	policyTagKeys := []tag.Key{tagPolicyKey}

	latencyDistributionAggregation := view.Distribution(1, 2, 5, 10, 25, 50, 75, 100, 150, 200, 300, 400, 500, 750, 1000, 2000, 3000, 4000, 5000, 10000, 20000, 30000, 50000)
	ageDistributionAggregation := view.Distribution(1, 2, 5, 10, 20, 30, 40, 50, 60, 90, 120, 180, 300, 600, 1800, 3600, 7200)

	views := make([]*view.View, 0)
	sampledTagKeys := []tag.Key{tagPolicyKey, tagSampledKey}
	views = append(views,
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statDecisionLatencyMicroSec.Name()),
			Measure:     statDecisionLatencyMicroSec,
			Description: statDecisionLatencyMicroSec.Description(),
			TagKeys:     policyTagKeys,
			Aggregation: latencyDistributionAggregation,
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statOverallDecisionLatencyUs.Name()),
			Measure:     statOverallDecisionLatencyUs,
			Description: statOverallDecisionLatencyUs.Description(),
			Aggregation: latencyDistributionAggregation,
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statTraceRemovalAgeSec.Name()),
			Measure:     statTraceRemovalAgeSec,
			Description: statTraceRemovalAgeSec.Description(),
			Aggregation: ageDistributionAggregation,
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statLateSpanArrivalAfterDecision.Name()),
			Measure:     statLateSpanArrivalAfterDecision,
			Description: statLateSpanArrivalAfterDecision.Description(),
			Aggregation: ageDistributionAggregation,
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statPolicyEvaluationErrorCount.Name()),
			Measure:     statPolicyEvaluationErrorCount,
			Description: statPolicyEvaluationErrorCount.Description(),
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statCountTracesSampled.Name()),
			Measure:     statCountTracesSampled,
			Description: statCountTracesSampled.Description(),
			TagKeys:     sampledTagKeys,
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statCountGlobalTracesSampled.Name()),
			Measure:     statCountGlobalTracesSampled,
			Description: statCountGlobalTracesSampled.Description(),
			TagKeys:     []tag.Key{tagSampledKey},
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statDroppedTooEarlyCount.Name()),
			Measure:     statDroppedTooEarlyCount,
			Description: statDroppedTooEarlyCount.Description(),
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statNewTraceIDReceivedCount.Name()),
			Measure:     statNewTraceIDReceivedCount,
			Description: statNewTraceIDReceivedCount.Description(),
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statTracesOnMemoryGauge.Name()),
			Measure:     statTracesOnMemoryGauge,
			Description: statTracesOnMemoryGauge.Description(),
			Aggregation: view.LastValue(),
		})

	if isMetricStatCountSpansSampledEnabled() {
		views = append(views, &view.View{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), statCountSpansSampled.Name()),
			Measure:     statCountSpansSampled,
			Description: statCountSpansSampled.Description(),
			TagKeys:     sampledTagKeys,
			Aggregation: view.Sum(),
		})
	}

	return views
}
