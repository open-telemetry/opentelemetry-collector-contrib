// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_exponential_histogram_ddl.sql
var metricsExponentialHistogramDDL string

// dMetricExponentialHistogram Exponential Histogram Metric to Doris
type dMetricExponentialHistogram struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Count                  int64          `json:"count"`
	Sum                    float64        `json:"sum"`
	Scale                  int32          `json:"scale"`
	ZeroCount              int64          `json:"zero_count"`
	PositiveOffset         int32          `json:"positive_offset"`
	PositiveBucketCounts   []int64        `json:"positive_bucket_counts"`
	NegativeOffset         int32          `json:"negative_offset"`
	NegativeBucketCounts   []int64        `json:"negative_bucket_counts"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	Min                    float64        `json:"min"`
	Max                    float64        `json:"max"`
	ZeroThreshold          float64        `json:"zero_threshold"`
	AggregationTemporality string         `json:"aggregation_temporality"`
}

type metricModelExponentialHistogram struct {
	data []*dMetricExponentialHistogram
}

func (m *metricModelExponentialHistogram) metricType() pmetric.MetricType {
	return pmetric.MetricTypeExponentialHistogram
}

func (m *metricModelExponentialHistogram) tableSuffix() string {
	return "_exponential_histogram"
}

func (m *metricModelExponentialHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("metric type is not exponential histogram: %v", pm.Type().String())
	}

	dataPoints := pm.ExponentialHistogram().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		exemplars := dp.Exemplars()
		newExemplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)

			newExemplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              e.getExemplarValue(exemplar),
				SpanID:             exemplar.SpanID().String(),
				TraceID:            exemplar.TraceID().String(),
			}

			newExemplars = append(newExemplars, newExemplar)
		}

		positiveBucketCounts := dp.Positive().BucketCounts()
		newPositiveBucketCounts := make([]int64, 0, positiveBucketCounts.Len())
		for j := 0; j < positiveBucketCounts.Len(); j++ {
			newPositiveBucketCounts = append(newPositiveBucketCounts, int64(positiveBucketCounts.At(j)))
		}

		negativeBucketCounts := dp.Negative().BucketCounts()
		newNegativeBucketCounts := make([]int64, 0, negativeBucketCounts.Len())
		for j := 0; j < negativeBucketCounts.Len(); j++ {
			newNegativeBucketCounts = append(newNegativeBucketCounts, int64(negativeBucketCounts.At(j)))
		}

		metric := &dMetricExponentialHistogram{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             dp.Attributes().AsRaw(),
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Count:                  int64(dp.Count()),
			Sum:                    dp.Sum(),
			Scale:                  dp.Scale(),
			ZeroCount:              int64(dp.ZeroCount()),
			PositiveOffset:         dp.Positive().Offset(),
			PositiveBucketCounts:   newPositiveBucketCounts,
			NegativeOffset:         dp.Negative().Offset(),
			NegativeBucketCounts:   newNegativeBucketCounts,
			Exemplars:              newExemplars,
			Min:                    dp.Min(),
			Max:                    dp.Max(),
			ZeroThreshold:          dp.ZeroThreshold(),
			AggregationTemporality: pm.ExponentialHistogram().AggregationTemporality().String(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}

func (m *metricModelExponentialHistogram) raw() any {
	return m.data
}

func (m *metricModelExponentialHistogram) size() int {
	return len(m.data)
}

func (m *metricModelExponentialHistogram) bytes() ([]byte, error) {
	return json.Marshal(m.data)
}
