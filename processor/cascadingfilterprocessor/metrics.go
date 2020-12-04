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

package cascadingfilterprocessor

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/obsreport"
)

// Variables related to metrics specific to Cascading Filter.
var (
	statusSampled              = "Sampled"
	statusNotSampled           = "NotSampled"
	statusExceededKey          = "RateExceeded"
	statusSecondChance         = "SecondChance"
	statusSecondChanceSampled  = "SecondChanceSampled"
	statusSecondChanceExceeded = "SecondChanceRateExceeded"

	tagPolicyKey, _                  = tag.NewKey("policy")
	tagCascadingFilterDecisionKey, _ = tag.NewKey("cascading_filter_decision")
	tagPolicyDecisionKey, _          = tag.NewKey("policy_decision")

	statDecisionLatencyMicroSec  = stats.Int64("policy_decision_latency", "Latency (in microseconds) of a given filtering policy", "µs")
	statOverallDecisionLatencyus = stats.Int64("cascading_filtering_batch_processing_latency", "Latency (in microseconds) of each run of the cascading filter timer", "µs")

	statTraceRemovalAgeSec           = stats.Int64("cascading_trace_removal_age", "Time (in seconds) from arrival of a new trace until its removal from memory", "s")
	statLateSpanArrivalAfterDecision = stats.Int64("cascadind_late_span_age", "Time (in seconds) from the cascading filter decision was taken and the arrival of a late span", "s")

	statPolicyEvaluationErrorCount = stats.Int64("cascading_policy_evaluation_error", "Count of cascading policy evaluation errors", stats.UnitDimensionless)

	statCascadingFilterDecision = stats.Int64("count_final_decision", "Count of traces that were filtered or not", stats.UnitDimensionless)
	statPolicyDecision          = stats.Int64("count_policy_decision", "Count of provisional (policy) decisions if traces were filtered or not", stats.UnitDimensionless)

	statDroppedTooEarlyCount    = stats.Int64("casdading_trace_dropped_too_early", "Count of traces that needed to be dropped the configured wait time", stats.UnitDimensionless)
	statNewTraceIDReceivedCount = stats.Int64("cascading_new_trace_id_received", "Counts the arrival of new traces", stats.UnitDimensionless)
	statTracesOnMemoryGauge     = stats.Int64("cascading_traces_on_memory", "Tracks the number of traces current on memory", stats.UnitDimensionless)
)

// CascadingFilterMetricViews return the metrics views according to given telemetry level.
func CascadingFilterMetricViews(level configtelemetry.Level) []*view.View {
	if level == configtelemetry.LevelNone {
		return nil
	}

	latencyDistributionAggregation := view.Distribution(1, 2, 5, 10, 25, 50, 75, 100, 150, 200, 300, 400, 500, 750, 1000, 2000, 3000, 4000, 5000, 10000, 20000, 30000, 50000)
	ageDistributionAggregation := view.Distribution(1, 2, 5, 10, 20, 30, 40, 50, 60, 90, 120, 180, 300, 600, 1800, 3600, 7200)

	overallDecisionLatencyView := &view.View{
		Name:        statOverallDecisionLatencyus.Name(),
		Measure:     statOverallDecisionLatencyus,
		Description: statOverallDecisionLatencyus.Description(),
		Aggregation: latencyDistributionAggregation,
	}

	traceRemovalAgeView := &view.View{
		Name:        statTraceRemovalAgeSec.Name(),
		Measure:     statTraceRemovalAgeSec,
		Description: statTraceRemovalAgeSec.Description(),
		Aggregation: ageDistributionAggregation,
	}

	lateSpanArrivalView := &view.View{
		Name:        statLateSpanArrivalAfterDecision.Name(),
		Measure:     statLateSpanArrivalAfterDecision,
		Description: statLateSpanArrivalAfterDecision.Description(),
		Aggregation: ageDistributionAggregation,
	}

	countPolicyEvaluationErrorView := &view.View{
		Name:        statPolicyEvaluationErrorCount.Name(),
		Measure:     statPolicyEvaluationErrorCount,
		Description: statPolicyEvaluationErrorCount.Description(),
		Aggregation: view.Sum(),
	}

	countFinalDecisionView := &view.View{
		Name:        statCascadingFilterDecision.Name(),
		Measure:     statCascadingFilterDecision,
		Description: statCascadingFilterDecision.Description(),
		TagKeys:     []tag.Key{tagPolicyKey, tagCascadingFilterDecisionKey},
		Aggregation: view.Sum(),
	}

	countPolicyDecisionsView := &view.View{
		Name:        statPolicyDecision.Name(),
		Measure:     statPolicyDecision,
		Description: statPolicyDecision.Description(),
		TagKeys:     []tag.Key{tagPolicyKey, tagPolicyDecisionKey},
		Aggregation: view.Sum(),
	}

	policyLatencyView := &view.View{
		Name:        statDecisionLatencyMicroSec.Name(),
		Measure:     statDecisionLatencyMicroSec,
		Description: statDecisionLatencyMicroSec.Description(),
		TagKeys:     []tag.Key{tagPolicyKey},
		Aggregation: view.Sum(),
	}

	countTraceDroppedTooEarlyView := &view.View{
		Name:        statDroppedTooEarlyCount.Name(),
		Measure:     statDroppedTooEarlyCount,
		Description: statDroppedTooEarlyCount.Description(),
		Aggregation: view.Sum(),
	}
	countTraceIDArrivalView := &view.View{
		Name:        statNewTraceIDReceivedCount.Name(),
		Measure:     statNewTraceIDReceivedCount,
		Description: statNewTraceIDReceivedCount.Description(),
		Aggregation: view.Sum(),
	}
	trackTracesOnMemorylView := &view.View{
		Name:        statTracesOnMemoryGauge.Name(),
		Measure:     statTracesOnMemoryGauge,
		Description: statTracesOnMemoryGauge.Description(),
		Aggregation: view.LastValue(),
	}

	legacyViews := []*view.View{
		overallDecisionLatencyView,
		traceRemovalAgeView,
		lateSpanArrivalView,

		countPolicyDecisionsView,
		policyLatencyView,
		countFinalDecisionView,

		countPolicyEvaluationErrorView,
		countTraceDroppedTooEarlyView,
		countTraceIDArrivalView,
		trackTracesOnMemorylView,
	}

	return obsreport.ProcessorMetricViews(typeStr, legacyViews)
}
