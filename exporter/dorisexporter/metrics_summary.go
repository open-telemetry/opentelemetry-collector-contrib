// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_summary_ddl.sql
var metricsSummaryDDL string

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
	return "_summary"
}

func (m *metricModelSummary) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter) error {
	if pm.Type() != pmetric.MetricTypeSummary {
		return fmt.Errorf("metric type is not summary: %v", pm.Type().String())
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
