// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/obsreport"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

var (
	mNumTracesConf      = stats.Int64("processor_groupbytrace_conf_num_traces", "Maximum number of traces to hold in the internal storage", stats.UnitDimensionless)
	mNumEventsInQueue   = stats.Int64("processor_groupbytrace_num_events_in_queue", "Number of events currently in the queue", stats.UnitDimensionless)
	mNumTracesInMemory  = stats.Int64("processor_groupbytrace_num_traces_in_memory", "Number of traces currently in the in-memory storage", stats.UnitDimensionless)
	mTracesEvicted      = stats.Int64("processor_groupbytrace_traces_evicted", "Traces evicted from the internal buffer", stats.UnitDimensionless)
	mReleasedSpans      = stats.Int64("processor_groupbytrace_spans_released", "Spans released to the next consumer", stats.UnitDimensionless)
	mReleasedTraces     = stats.Int64("processor_groupbytrace_traces_released", "Traces released to the next consumer", stats.UnitDimensionless)
	mIncompleteReleases = stats.Int64("processor_groupbytrace_incomplete_releases", "Releases that are suspected to have been incomplete", stats.UnitDimensionless)
	mEventLatency       = stats.Int64("processor_groupbytrace_event_latency", "How long the queue events are taking to be processed", stats.UnitMilliseconds)
)

// MetricViews return the metrics views according to given telemetry level.
func MetricViews() []*view.View {
	return []*view.View{
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mNumTracesConf.Name()),
			Measure:     mNumTracesConf,
			Description: mNumTracesConf.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mNumEventsInQueue.Name()),
			Measure:     mNumEventsInQueue,
			Description: mNumEventsInQueue.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mNumTracesInMemory.Name()),
			Measure:     mNumTracesInMemory,
			Description: mNumTracesInMemory.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mTracesEvicted.Name()),
			Measure:     mTracesEvicted,
			Description: mTracesEvicted.Description(),
			// sum allows us to start from 0, count will only show up if there's at least one eviction, which might take a while to happen (if ever!)
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mReleasedSpans.Name()),
			Measure:     mReleasedSpans,
			Description: mReleasedSpans.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mReleasedTraces.Name()),
			Measure:     mReleasedTraces,
			Description: mReleasedTraces.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mIncompleteReleases.Name()),
			Measure:     mIncompleteReleases,
			Description: mIncompleteReleases.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(metadata.Type), mEventLatency.Name()),
			Measure:     mEventLatency,
			Description: mEventLatency.Description(),
			TagKeys: []tag.Key{
				tag.MustNewKey("event"),
			},
			Aggregation: view.Distribution(0, 5, 10, 20, 50, 100, 200, 500, 1000),
		},
	}
}
