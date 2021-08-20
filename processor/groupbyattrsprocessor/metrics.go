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

package groupbyattrsprocessor

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/obsreport"
)

var (
	mNumGroupedSpans    = stats.Int64("num_grouped_spans", "Number of spans that had attributes grouped", stats.UnitDimensionless)
	mNumNonGroupedSpans = stats.Int64("num_non_grouped_spans", "Number of spans that did not have attributes grouped", stats.UnitDimensionless)
	mDistSpanGroups     = stats.Int64("span_groups", "Distributon of groups extracted for spans", stats.UnitDimensionless)
	mNumGroupedLogs     = stats.Int64("num_grouped_logs", "Number of logs that had attributes grouped", stats.UnitDimensionless)
	mNumNonGroupedLogs  = stats.Int64("num_non_grouped_logs", "Number of logs that did not have attributes grouped", stats.UnitDimensionless)
	mDistLogGroups      = stats.Int64("log_groups", "Distributon of groups extracted for logs", stats.UnitDimensionless)
)

// MetricViews return the metrics views according to given telemetry level.
func MetricViews() []*view.View {
	distributionGroups := view.Distribution(1, 2, 5, 10, 20, 50, 100, 500, 2000)

	return []*view.View{
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mNumGroupedSpans.Name()),
			Measure:     mNumGroupedSpans,
			Description: mNumGroupedSpans.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mNumNonGroupedSpans.Name()),
			Measure:     mNumNonGroupedSpans,
			Description: mNumNonGroupedSpans.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mDistSpanGroups.Name()),
			Measure:     mDistSpanGroups,
			Description: mDistSpanGroups.Description(),
			Aggregation: distributionGroups,
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mNumGroupedLogs.Name()),
			Measure:     mNumGroupedLogs,
			Description: mNumGroupedLogs.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mNumNonGroupedLogs.Name()),
			Measure:     mNumNonGroupedLogs,
			Description: mNumNonGroupedLogs.Description(),
			Aggregation: view.Sum(),
		},
		{
			Name:        obsreport.BuildProcessorCustomMetricName(string(typeStr), mDistLogGroups.Name()),
			Measure:     mDistLogGroups,
			Description: mDistLogGroups.Description(),
			Aggregation: distributionGroups,
		},
	}
}
