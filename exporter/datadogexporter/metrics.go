// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"context"
	"fmt"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

type MetricsExporter interface {
	PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error)
	GetLogger() *zap.Logger
	GetConfig() *Config
	GetQueueSettings() exporterhelper.QueueSettings
	GetRetrySettings() exporterhelper.RetrySettings
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) (MetricsExporter, error) {
	switch cfg.Mode {
	case AgentMode:
		return newDogStatsDExporter(logger, cfg)
	}

	return nil, fmt.Errorf("Unsupported mode: '%s'", cfg.Mode)
}

type MetricType int

const (
	Count MetricType = iota
	Gauge
)

type Metric struct {
	hostname   string
	name       string
	metricType MetricType
	fvalue     float64
	ivalue     int64
	tags       []string
	rate       float64
}

func (m *Metric) GetHost() string {
	return m.hostname
}

func (m *Metric) GetName() string {
	return m.name
}

func (m *Metric) GetType() MetricType {
	return m.metricType
}

func (m *Metric) GetValue() interface{} {
	switch m.metricType {
	case Count:
		return m.ivalue
	case Gauge:
		return m.fvalue
	}
	return nil
}

func (m *Metric) GetRate() float64 {
	return m.rate
}

func (m *Metric) GetTags() []string {
	return m.tags
}

func NewMetric(hostname, name string, metricType MetricType, value interface{}, tags []string, rate float64) (*Metric, error) {
	switch metricType {
	case Count:
		ivalue, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("Incorrect value type for count metric '%s'", name)
		}
		return &Metric{
			hostname:   hostname,
			name:       name,
			metricType: Count,
			ivalue:     ivalue,
			rate:       rate,
			tags:       tags,
		}, nil

	case Gauge:
		fvalue, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("Incorrect value type for count metric '%s'", name)
		}
		return &Metric{
			hostname:   hostname,
			name:       name,
			metricType: Gauge,
			fvalue:     fvalue,
			rate:       rate,
			tags:       tags,
		}, nil
	}

	return nil, fmt.Errorf("Unrecognized Metric type for metric '%s'", name)
}

type OpenCensusKind int

const (
	Int64 OpenCensusKind = iota
	Double
	Distribution
	Summary
)

func MapMetrics(exp MetricsExporter, md pdata.Metrics) (map[string][]*Metric, int, error) {
	// Transform it into OpenCensus format
	data := pdatautil.MetricsToMetricsData(md)

	// Mapping from metrics name to data
	metrics := map[string][]*Metric{}

	logger := exp.GetLogger()

	// The number of timeseries we drop (required by pushMetricsData interface)
	var droppedTimeSeries = 0

	for _, metricsData := range data {
		// The hostname provided by OpenTelemetry
		hostname := metricsData.Node.GetIdentifier().GetHostName()

		for _, metric := range metricsData.Metrics {

			// The metric name
			metricName := metric.GetMetricDescriptor().GetName()
			// For logging purposes
			nameField := zap.String("name", metricName)

			// Labels are divided in keys and values, keys are shared among timeseries.
			// We transform them into Datadog tags of the form key:value
			labelKeys := metric.GetMetricDescriptor().GetLabelKeys()

			// Get information about metric
			// We ignore whether the metric is cumulative or not
			var kind OpenCensusKind
			switch metric.GetMetricDescriptor().GetType() {
			case v1.MetricDescriptor_GAUGE_INT64, v1.MetricDescriptor_CUMULATIVE_INT64:
				kind = Int64
			case v1.MetricDescriptor_GAUGE_DOUBLE, v1.MetricDescriptor_CUMULATIVE_DOUBLE:
				kind = Double
			case v1.MetricDescriptor_GAUGE_DISTRIBUTION, v1.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
				kind = Distribution
			case v1.MetricDescriptor_SUMMARY:
				kind = Summary
			default:
				logger.Info(
					"Discarding metric: unspecified or unrecognized type",
					nameField,
					zap.Any("type", metric.GetMetricDescriptor().GetType()),
				)
				droppedTimeSeries += len(metric.GetTimeseries())
				continue
			}

			for _, timeseries := range metric.GetTimeseries() {

				// Create tags
				labelValues := timeseries.GetLabelValues()
				tags := make([]string, len(labelKeys))
				for i, key := range labelKeys {
					labelValue := labelValues[i]
					if labelValue.GetHasValue() {
						// Tags can't end with ":" so we replace empty values with "n/a"
						value := labelValue.GetValue()
						if value == "" {
							value = "n/a"
						}

						tags[i] = fmt.Sprintf("%s:%s", key.GetKey(), value)
					}
				}

				for _, point := range timeseries.GetPoints() {
					// We assume the sampling rate is 1.
					const defaultRate float64 = 1

					switch kind {
					case Int64, Double:
						var value float64
						if kind == Int64 {
							value = float64(point.GetInt64Value())
						} else {
							value = point.GetDoubleValue()
						}

						newVal, err := NewMetric(hostname, metricName, Gauge, value, tags, defaultRate)
						if err != nil {
							logger.Error("Error when creating Datadog metric, continuing...", nameField, zap.Error(err))
							continue
						}
						metrics[metricName] = append(metrics[metricName], newVal)
					case Distribution:
						logger.Warn("Ignoring distribution metric, not implemented yet", nameField)
						continue
					case Summary:
						logger.Warn("Ignoring summary metric, not implemented yet", nameField)
						continue
					}
				}
			}
		}
	}

	return metrics, droppedTimeSeries, nil
}
