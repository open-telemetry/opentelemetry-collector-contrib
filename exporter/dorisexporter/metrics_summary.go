// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	metricsSummaryTableSuffix = "_summary"
	metricsSummaryDDL         = `
CREATE TABLE IF NOT EXISTS %s` + metricsSummaryTableSuffix + `
(
    service_name          VARCHAR(200),
    timestamp             DATETIME(6),
    metric_name           VARCHAR(200),
    metric_description    STRING,
    metric_unit           STRING,
    attributes            VARIANT,
    start_time            DATETIME(6),
    count                 BIGINT,
    sum                   DOUBLE,
    quantile_values       ARRAY<STRUCT<quantile:DOUBLE, value:DOUBLE>>,
    resource_attributes   VARIANT,
    scope_name            STRING,
    scope_version         STRING,

    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_metric_name(metric_name) USING INVERTED,
    INDEX idx_metric_description(metric_description) USING INVERTED,
    INDEX idx_metric_unit(metric_unit) USING INVERTED,
    INDEX idx_attributes(attributes) USING INVERTED,
    INDEX idx_start_time(start_time) USING INVERTED,
    INDEX idx_count(count) USING INVERTED,
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

// dMetricSummary Summary metric model to Doris
type dMetricSummary struct {
	*dMetric       `json:",inline"`
	Timestamp      string            `json:"timestamp"`
	Attributes     map[string]any    `json:"attributes"`
	StartTime      string            `json:"start_time"`
	Count          int64             `json:"count"`
	Sum            float64           `json:"sum"`
	QuantileValues []*dQuantileValue `json:"quantile_values"`
}

// dQuantileValue Quantile Value to Doris
type dQuantileValue struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

type metricModelSummary struct {
	data []*dMetricSummary
}

func (m *metricModelSummary) metricType() pmetric.MetricType {
	return pmetric.MetricTypeSummary
}

func (m *metricModelSummary) tableSuffix() string {
	return metricsSummaryTableSuffix
}

func (m *metricModelSummary) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeSummary {
		return fmt.Errorf("metric type is not Summary: %v", pm.Type().String())
	}

	dataPoints := pm.Summary().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		quantileValues := dp.QuantileValues()
		newQuantileValues := make([]*dQuantileValue, 0, quantileValues.Len())
		for j := 0; j < quantileValues.Len(); j++ {
			quantileValue := quantileValues.At(j)

			newQuantileValue := &dQuantileValue{
				Quantile: quantileValue.Quantile(),
				Value:    quantileValue.Value(),
			}

			newQuantileValues = append(newQuantileValues, newQuantileValue)
		}

		metric := &dMetricSummary{
			dMetric:        dm,
			Timestamp:      e.formatTime(dp.Timestamp().AsTime()),
			Attributes:     dp.Attributes().AsRaw(),
			StartTime:      e.formatTime(dp.StartTimestamp().AsTime()),
			Count:          int64(dp.Count()),
			Sum:            dp.Sum(),
			QuantileValues: newQuantileValues,
		}
		m.data = append(m.data, metric)
	}

	return nil
}

func (m *metricModelSummary) raw() any {
	return m.data
}

func (m *metricModelSummary) size() int {
	return len(m.data)
}

func (m *metricModelSummary) bytes() ([]byte, error) {
	return json.Marshal(m.data)
}
