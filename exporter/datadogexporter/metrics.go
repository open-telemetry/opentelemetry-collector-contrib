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
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"
)

type MetricsExporter interface {
	PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error)
	GetLogger() *zap.Logger
	GetConfig() *Config
	GetQueueSettings() exporterhelper.QueueSettings
	GetRetrySettings() exporterhelper.RetrySettings
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) (MetricsExporter, error) {
	switch cfg.Metrics.Mode {
	case DogStatsDMode:
		return newDogStatsDExporter(logger, cfg)
	case AgentlessMode:
		return newMetricsAPIExporter(logger, cfg)
	case NoneMode:
		return nil, fmt.Errorf("metrics exporter disabled for Datadog exporter")
	}

	return nil, fmt.Errorf("unsupported mode: '%s'", cfg.Metrics.Mode)
}

const (
	Gauge string = "gauge"
)

func NewGauge(hostname, name string, ts int32, value float64, tags []string) datadog.Metric {
	timestamp := float64(ts)

	gauge := datadog.Metric{
		Points: []datadog.DataPoint{[2]*float64{&timestamp, &value}},
		Tags:   tags,
	}
	gauge.SetHost(hostname)
	gauge.SetMetric(name)
	gauge.SetType(Gauge)
	return gauge
}

type OpenCensusKind int

const (
	Int64 OpenCensusKind = iota
	Double
	Distribution
	Summary
)

// Series is a set of metrics
type Series struct {
	metrics []datadog.Metric
}

func (m *Series) Add(metric datadog.Metric) {
	m.metrics = append(m.metrics, metric)
}

func MapMetrics(exp MetricsExporter, data []consumerdata.MetricsData) (series Series, droppedTimeSeries int) {
	logger := exp.GetLogger()

	for _, metricsData := range data {
		// The hostname provided by OpenTelemetry
		host := metricsData.Node.GetIdentifier().GetHostName()

		for _, metric := range metricsData.Metrics {

			// The metric name
			name := metric.GetMetricDescriptor().GetName()
			// For logging purposes
			nameField := zap.String("name", name)

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
					ts := int32(point.Timestamp.Seconds)

					switch kind {
					case Int64:
						series.Add(NewGauge(host, name, ts, float64(point.GetInt64Value()), tags))
					case Double:
						series.Add(NewGauge(host, name, ts, point.GetDoubleValue(), tags))
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
							fullName := fmt.Sprintf("%s.%s", name, suffix)
							series.Add(NewGauge(host, fullName, ts, value, tags))
						}

						if exp.GetConfig().Metrics.Buckets {
							// We have a single metric, 'count_per_bucket', which is tagged with the bucket id. See:
							// https://github.com/DataDog/opencensus-go-exporter-datadog/blob/c3b47f1c6dcf1c47b59c32e8dbb7df5f78162daa/stats.go#L99-L104
							fullName := fmt.Sprintf("%s.count_per_bucket", name)

							for idx, bucket := range dist.GetBuckets() {
								bucketTags := append(tags, fmt.Sprintf("bucket_idx:%d", idx))
								series.Add(NewGauge(host, fullName, ts, float64(bucket.GetCount()), bucketTags))
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
							fullName := fmt.Sprintf("%s.count", name)
							series.Add(NewGauge(host, fullName, ts, float64(count.GetValue()), tags))
						}

						// Report sum if available
						if sum := point.GetSummaryValue().GetSum(); sum != nil {
							fullName := fmt.Sprintf("%s.sum", name)
							series.Add(NewGauge(host, fullName, ts, sum.GetValue(), tags))
						}

						if exp.GetConfig().Metrics.Percentiles {
							snapshot := point.GetSummaryValue().GetSnapshot()
							for _, pair := range snapshot.GetPercentileValues() {
								var fullName string
								if perc := pair.GetPercentile(); perc == 0 {
									// p0 is the minimum
									fullName = fmt.Sprintf("%s.min", name)
								} else if perc == 100 {
									// p100 is the maximum
									fullName = fmt.Sprintf("%s.max", name)
								} else {
									// Round to the nearest digit
									fullName = fmt.Sprintf("%s.p%02d", name, int(math.Round(perc)))
								}

								series.Add(NewGauge(host, fullName, ts, pair.GetValue(), tags))
							}
						}

					}
				}
			}
		}
	}

	return
}
