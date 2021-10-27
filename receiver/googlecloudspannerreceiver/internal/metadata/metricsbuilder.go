// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"sort"

	"go.opentelemetry.io/collector/model/pdata"
)

const instrumentationLibraryName = "otelcol/googlecloudspannermetrics"

type MetricsBuilder struct {
	// TODO It is empty now but there will be fields soon
}

func (b *MetricsBuilder) Build(dataPoints []*MetricsDataPoint) pdata.Metrics {
	var metrics pdata.Metrics

	groupedDataPoints := group(dataPoints)

	if len(groupedDataPoints) > 0 {
		metrics = pdata.NewMetrics()
		rms := metrics.ResourceMetrics()
		rm := rms.AppendEmpty()

		ilms := rm.InstrumentationLibraryMetrics()
		ilm := ilms.AppendEmpty()
		ilm.InstrumentationLibrary().SetName(instrumentationLibraryName)

		for key, points := range groupedDataPoints {
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(key.MetricName)
			metric.SetUnit(key.MetricUnit)
			metric.SetDataType(key.MetricDataType.MetricDataType())

			var dataPointSlice pdata.NumberDataPointSlice

			switch key.MetricDataType.MetricDataType() {
			case pdata.MetricDataTypeGauge:
				dataPointSlice = metric.Gauge().DataPoints()
			case pdata.MetricDataTypeSum:
				metric.Sum().SetAggregationTemporality(key.MetricDataType.AggregationTemporality())
				metric.Sum().SetIsMonotonic(key.MetricDataType.IsMonotonic())
				dataPointSlice = metric.Sum().DataPoints()
			}

			for _, point := range points {
				point.CopyTo(dataPointSlice.AppendEmpty())
			}
		}
	}

	return metrics
}

func group(dataPoints []*MetricsDataPoint) map[MetricsDataPointKey][]*MetricsDataPoint {
	if len(dataPoints) == 0 {
		return nil
	}

	groupedDataPoints := make(map[MetricsDataPointKey][]*MetricsDataPoint)

	for _, dataPoint := range dataPoints {
		groupingKey := dataPoint.GroupingKey()
		groupedDataPoints[groupingKey] = append(groupedDataPoints[groupingKey], dataPoint)
	}

	// TODO Apply cardinality filtering somewhere here for each group of dataPoints and rename method to groupAndFilter()

	// Sorting points by metric name and timestamp
	for _, points := range groupedDataPoints {
		sort.Slice(points, func(i, j int) bool {
			return points[i].timestamp.Before(points[j].timestamp)
		})

	}

	return groupedDataPoints
}
