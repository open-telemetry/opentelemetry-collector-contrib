// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	metricsExponentialHistogramTableSuffix = "_exponential_histogram"
	metricsExponentialHistogramDDL         = `
CREATE TABLE IF NOT EXISTS %s` + metricsExponentialHistogramTableSuffix + `
(
    service_name            VARCHAR(200),
    timestamp               DATETIME(6),
    metric_name             VARCHAR(200),
    metric_description      STRING,
    metric_unit             STRING,
    attributes              VARIANT,
    start_time              DATETIME(6),
    count                   BIGINT,
    sum                     DOUBLE,
    scale                   INT,
    zero_count              BIGINT,
    positive_offset         INT,
    positive_bucket_counts  ARRAY<BIGINT>,
    negative_offset         INT,
    negative_bucket_counts  ARRAY<BIGINT>,
    exemplars               ARRAY<STRUCT<filtered_attributes:MAP<STRING,STRING>, timestamp:DATETIME(6), value:DOUBLE, span_id:STRING, trace_id:STRING>>,
    min                     DOUBLE,
    max                     DOUBLE,
    zero_threshold          DOUBLE,
    aggregation_temporality STRING,
    resource_attributes     VARIANT,
    scope_name              STRING,
    scope_version           STRING,

    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_metric_name(metric_name) USING INVERTED,
    INDEX idx_metric_description(metric_description) USING INVERTED,
    INDEX idx_metric_unit(metric_unit) USING INVERTED,
    INDEX idx_attributes(attributes) USING INVERTED,
    INDEX idx_start_time(start_time) USING INVERTED,
    INDEX idx_count(count) USING INVERTED,
    INDEX idx_scale(scale) USING INVERTED,
    INDEX idx_zero_count(zero_count) USING INVERTED,
    INDEX idx_positive_offset(positive_offset) USING INVERTED,
    INDEX idx_negative_offset(negative_offset) USING INVERTED,
    INDEX idx_aggregation_temporality(aggregation_temporality) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(metric_name) BUCKETS AUTO
%s;
`
)

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
	return metricsExponentialHistogramTableSuffix
}

func (m *metricModelExponentialHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("metric type is not Exponential Histogram: %v", pm.Type().String())
	}

	dataPoints := pm.ExponentialHistogram().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		exemplars := dp.Exemplars()
		newExeplars := make([]*dExemplar, 0, exemplars.Len())
		for j := 0; j < exemplars.Len(); j++ {
			exemplar := exemplars.At(j)

			newExeplar := &dExemplar{
				FilteredAttributes: exemplar.FilteredAttributes().AsRaw(),
				Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
				Value:              e.getExemplarValue(exemplar),
				SpanID:             exemplar.SpanID().String(),
				TraceID:            exemplar.TraceID().String(),
			}

			newExeplars = append(newExeplars, newExeplar)
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
			Exemplars:              newExeplars,
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
