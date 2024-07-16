// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

var (
	mNumTracesConf      = stats.Int64("conf_num_traces", "Maximum number of traces to hold in the internal storage", stats.UnitDimensionless)
	mNumEventsInQueue   = stats.Int64("num_events_in_queue", "Number of events currently in the queue", stats.UnitDimensionless)
	mNumTracesInMemory  = stats.Int64("num_traces_in_memory", "Number of traces currently in the in-memory storage", stats.UnitDimensionless)
	mTracesEvicted      = stats.Int64("traces_evicted", "Traces evicted from the internal buffer", stats.UnitDimensionless)
	mReleasedSpans      = stats.Int64("spans_released", "Spans released to the next consumer", stats.UnitDimensionless)
	mReleasedTraces     = stats.Int64("traces_released", "Traces released to the next consumer", stats.UnitDimensionless)
	mIncompleteReleases = stats.Int64("incomplete_releases", "Releases that are suspected to have been incomplete", stats.UnitDimensionless)
	mEventLatency       = stats.Int64("event_latency", "How long the queue events are taking to be processed", stats.UnitMilliseconds)
)

// metricViews return the metrics views according to given telemetry level.
func metricViews() []*view.View {
	return []*view.View{
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mNumTracesConf.Name()),
			Measure:     mNumTracesConf,
			Description: mNumTracesConf.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mNumEventsInQueue.Name()),
			Measure:     mNumEventsInQueue,
			Description: mNumEventsInQueue.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mNumTracesInMemory.Name()),
			Measure:     mNumTracesInMemory,
			Description: mNumTracesInMemory.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mTracesEvicted.Name()),
			Measure:     mTracesEvicted,
			Description: mTracesEvicted.Description(),
			// sum allows us to start from 0, count will only show up if there's at least one eviction, which might take a while to happen (if ever!)
			Aggregation: view.Sum(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mReleasedSpans.Name()),
			Measure:     mReleasedSpans,
			Description: mReleasedSpans.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mReleasedTraces.Name()),
			Measure:     mReleasedTraces,
			Description: mReleasedTraces.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mIncompleteReleases.Name()),
			Measure:     mIncompleteReleases,
			Description: mIncompleteReleases.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        processorhelper.BuildCustomMetricName(metadata.Type.String(), mEventLatency.Name()),
			Measure:     mEventLatency,
			Description: mEventLatency.Description(),
			TagKeys: []tag.Key{
				tag.MustNewKey("event"),
			},
			Aggregation: view.Distribution(0, 5, 10, 20, 50, 100, 200, 500, 1000),
		},
	}
}
