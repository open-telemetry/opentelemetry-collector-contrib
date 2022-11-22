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

package model

import (
	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricsInner struct {
	Gauges         map[string]float64 `json:"gauges,omitempty"`
	HistogramsMean map[string]float64 `json:"histograms_mean,omitempty"`
	Sums           map[string]float64 `json:"sums,omitempty"`
}

func newMetricsInner() metricsInner {
	return metricsInner{
		Gauges:         make(map[string]float64),
		HistogramsMean: make(map[string]float64),
		Sums:           make(map[string]float64),
	}
}

type OpenTelemetryCustomMetricsData struct {
	Metrics metricsInner `json:"metrics,omitempty"`
	Pid     string       `json:"pid,omitempty"`
}

func NewOpenTelemetryCustomMetricsData() OpenTelemetryCustomMetricsData {
	return OpenTelemetryCustomMetricsData{
		Metrics: newMetricsInner(),
	}
}

func (omData *OpenTelemetryCustomMetricsData) AppendMetric(metric pmetric.Metric) {
	metricName := metric.Name()

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for j := 0; j < metric.Gauge().DataPoints().Len(); j++ {
			dp := metric.Gauge().DataPoints().At(j)

			if dp.ValueType() == pmetric.NumberDataPointValueTypeDouble {
				omData.Metrics.Gauges[metricNameToCompact(metricName, dp.Attributes())] = dp.DoubleValue()
			}

			if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
				omData.Metrics.Gauges[metricNameToCompact(metricName, dp.Attributes())] = float64(dp.IntValue())
			}
		}
	case pmetric.MetricTypeSum:
		for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
			dp := metric.Sum().DataPoints().At(j)

			if dp.ValueType() == pmetric.NumberDataPointValueTypeDouble {
				omData.Metrics.Sums[metricNameToCompact(metricName, dp.Attributes())] = dp.DoubleValue()
			}

			if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
				omData.Metrics.Sums[metricNameToCompact(metricName, dp.Attributes())] = float64(dp.IntValue())
			}
		}
	case pmetric.MetricTypeHistogram:
		for j := 0; j < metric.Histogram().DataPoints().Len(); j++ {
			dp := metric.Histogram().DataPoints().At(j)

			omData.Metrics.HistogramsMean[metricNameToCompact(metricName, dp.Attributes())] = dp.Sum()
		}
	}
}

func NewOpenTelemetryMetricsPluginPayload(entityID string, data OpenTelemetryCustomMetricsData) instanaacceptor.PluginPayload {
	const pluginName = "com.instana.plugin.otel.metrics"

	return instanaacceptor.PluginPayload{
		Name:     pluginName,
		EntityID: entityID,
		Data:     data,
	}
}
