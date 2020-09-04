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
	"math"

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
	// A different exporter will be added depending on configuration
	// in the future
	return newDogStatsDExporter(logger, cfg)
}

type MetricType int

const (
	Gauge MetricType = iota
)

type MetricValue struct {
	hostname   string
	metricType MetricType
	value      float64
	tags       []string
	rate       float64
}

func (m *MetricValue) GetHost() string {
	return m.hostname
}

func (m *MetricValue) GetType() MetricType {
	return m.metricType
}

func (m *MetricValue) GetValue() float64 {
	return m.value
}

func (m *MetricValue) GetRate() float64 {
	return m.rate
}

func (m *MetricValue) GetTags() []string {
	return m.tags
}

func NewGauge(hostname string, value float64, tags []string) MetricValue {
	return MetricValue{
		hostname:   hostname,
		metricType: Gauge,
		value:      value,
		rate:       1,
		tags:       tags,
	}
}

type OpenCensusKind int

const (
	Int64 OpenCensusKind = iota
	Double
	Distribution
	Summary
)

func MapMetrics(exp MetricsExporter, md pdata.Metrics) (map[string][]MetricValue, int) {
	// Transform it into OpenCensus format
	data := pdatautil.MetricsToMetricsData(md)

	// Mapping from metrics name to data
	metrics := map[string][]MetricValue{}

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
					switch kind {
					case Int64:
						metrics[metricName] = append(metrics[metricName],
							NewGauge(hostname, float64(point.GetInt64Value()), tags),
						)
					case Double:
						metrics[metricName] = append(metrics[metricName],
							NewGauge(hostname, point.GetDoubleValue(), tags),
						)
					case Distribution:
						// A Distribution metric has:
						// - The count of values in the population
						// - The sum of values in the population
						// - The sum of squared deviations
						// - A number of buckets, each of them having
						//    - the bounds that define the bucket
						//    - the count of the number of items in that bucket
						//    - a sample value from each bucket
						//
						// We follow the implementation on `opencensus-go-exporter-datadog`:
						// we report the first three values and the buckets count can also
						// be reported (opt-in), but bounds are ignored.

						dist := point.GetDistributionValue()

						distMetrics := map[string]float64{
							"count":           float64(dist.GetCount()),
							"sum":             dist.GetSum(),
							"squared_dev_sum": dist.GetSumOfSquaredDeviation(),
						}

						for suffix, value := range distMetrics {
							fullName := fmt.Sprintf("%s.%s", metricName, suffix)
							metrics[fullName] = append(metrics[fullName],
								NewGauge(hostname, value, tags),
							)
						}

						if exp.GetConfig().Metrics.Buckets {
							// We have a single metric, 'count_per_bucket', which is tagged with the bucket id. See:
							// https://github.com/DataDog/opencensus-go-exporter-datadog/blob/c3b47f1c6dcf1c47b59c32e8dbb7df5f78162daa/stats.go#L99-L104
							fullName := fmt.Sprintf("%s.count_per_bucket", metricName)

							for idx, bucket := range dist.GetBuckets() {
								bucketTags := append(tags, fmt.Sprintf("bucket_idx:%d", idx))
								metrics[fullName] = append(metrics[fullName],
									NewGauge(hostname, float64(bucket.GetCount()), bucketTags),
								)
							}
						}
					case Summary:
						// A Summary metric has:
						// - The total sum so far
						// - The total count so far
						// - A snapshot with
						//   - the sum in the current snapshot
						//   - the count in the current snapshot
						//   - a series of percentiles
						//
						// By default we report the sum and count as gauges and percentiles.
						// Percentiles are opt-out

						// Report count if available
						if count := point.GetSummaryValue().GetCount(); count != nil {
							fullName := fmt.Sprintf("%s.count", metricName)
							metrics[fullName] = append(metrics[fullName],
								NewGauge(hostname, float64(count.GetValue()), tags),
							)
						}

						// Report sum if available
						if sum := point.GetSummaryValue().GetSum(); sum != nil {
							fullName := fmt.Sprintf("%s.sum", metricName)
							metrics[fullName] = append(metrics[fullName],
								NewGauge(hostname, sum.GetValue(), tags),
							)
						}

						if exp.GetConfig().Metrics.Percentiles {
							snapshot := point.GetSummaryValue().GetSnapshot()
							for _, pair := range snapshot.GetPercentileValues() {
								var fullName string
								if perc := pair.GetPercentile(); perc == 0 {
									// p0 is the minimum
									fullName = fmt.Sprintf("%s.min", metricName)
								} else if perc == 100 {
									// p100 is the maximum
									fullName = fmt.Sprintf("%s.max", metricName)
								} else {
									// Round to the nearest digit
									fullName = fmt.Sprintf("%s.p%02d", metricName, int(math.Round(perc)))
								}

								metrics[fullName] = append(metrics[fullName],
									NewGauge(hostname, pair.GetValue(), tags),
								)
							}
						}

					}
				}
			}
		}
	}

	return metrics, droppedTimeSeries
}
