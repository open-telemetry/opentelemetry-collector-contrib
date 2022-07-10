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

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/obsreport"
)

var (
	mNumLogsConf        = stats.Int64("processor_groupbytrace_conf_num_logs", "Maximum number of logs to hold in the internal storage", stats.UnitDimensionless)
	mNumEventsInQueue   = stats.Int64("processor_groupbytrace_num_events_in_queue", "Number of events currently in the queue", stats.UnitDimensionless)
	mNumLogsInMemory    = stats.Int64("processor_groupbytrace_num_logs_in_memory", "Number of logs currently in the in-memory storage", stats.UnitDimensionless)
	mLogsEvicted        = stats.Int64("processor_groupbytrace_logs_evicted", "Logs evicted from the internal buffer", stats.UnitDimensionless)
	mReleasedLogRecords = stats.Int64("processor_groupbytrace_logRecords_released", "LogRecords released to the next consumer", stats.UnitDimensionless)
	mReleasedLogs       = stats.Int64("processor_groupbytrace_logs_released", "Logs released to the next consumer", stats.UnitDimensionless)
	mIncompleteReleases = stats.Int64("processor_groupbytrace_incomplete_releases", "Releases that are suspected to have been incomplete", stats.UnitDimensionless)
	mEventLatency       = stats.Int64("processor_groupbytrace_event_latency", "How long the queue events are taking to be processed", stats.UnitMilliseconds)
)

// MetricViews return the metrics views according to given telemetry level.
func MetricViews() []*view.View {
	return []*view.View{
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mNumLogsConf.Name()),
			Measure:     mNumLogsConf,
			Description: mNumLogsConf.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mNumEventsInQueue.Name()),
			Measure:     mNumEventsInQueue,
			Description: mNumEventsInQueue.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mNumLogsInMemory.Name()),
			Measure:     mNumLogsInMemory,
			Description: mNumLogsInMemory.Description(),
			Aggregation: view.LastValue(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mLogsEvicted.Name()),
			Measure:     mLogsEvicted,
			Description: mLogsEvicted.Description(),
			// sum allows us to start from 0, count will only show up if there's at least one eviction, which might take a while to happen (if ever!)
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mReleasedLogRecords.Name()),
			Measure:     mReleasedLogRecords,
			Description: mReleasedLogRecords.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mReleasedLogs.Name()),
			Measure:     mReleasedLogs,
			Description: mReleasedLogs.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mIncompleteReleases.Name()),
			Measure:     mIncompleteReleases,
			Description: mIncompleteReleases.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string("groupbytrace"), mEventLatency.Name()),
			Measure:     mEventLatency,
			Description: mEventLatency.Description(),
			TagKeys: []tag.Key{
				tag.MustNewKey("event"),
			},
			Aggregation: view.Distribution(0, 5, 10, 20, 50, 100, 200, 500, 1000),
		},
	}
}
