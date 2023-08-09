// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/obsreport"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

// Variables related to metrics specific to tail sampling.
var (
	tagPolicyKey, _  = tag.NewKey("policy")
	tagSampledKey, _ = tag.NewKey("sampled")

	statCountTracesSampled = stats.Int64("count_traces_sampled", "Count of traces that were sampled or not", stats.UnitDimensionless)
	statCountLogsSampled   = stats.Int64("count_logs_sampled", "Count of logs that were sampled or not", stats.UnitDimensionless)
)

// SamplingProcessorMetricViews return the metrics views according to given telemetry level.
func SamplingProcessorMetricViews(level configtelemetry.Level) []*view.View {
	if level == configtelemetry.LevelNone {
		return nil
	}

	sampledTagKeys := []tag.Key{tagPolicyKey, tagSampledKey}
	countTracesSampledView := &view.View{
		Name:        obsreport.BuildProcessorCustomMetricName(metadata.Type, statCountTracesSampled.Name()),
		Measure:     statCountTracesSampled,
		Description: statCountTracesSampled.Description(),
		TagKeys:     sampledTagKeys,
		Aggregation: view.Sum(),
	}
	countLogsSampledView := &view.View{
		Name:        obsreport.BuildProcessorCustomMetricName(metadata.Type, statCountLogsSampled.Name()),
		Measure:     statCountLogsSampled,
		Description: statCountLogsSampled.Description(),
		TagKeys:     sampledTagKeys,
		Aggregation: view.Sum(),
	}

	return []*view.View{
		countTracesSampledView,
		countLogsSampledView,
	}
}
