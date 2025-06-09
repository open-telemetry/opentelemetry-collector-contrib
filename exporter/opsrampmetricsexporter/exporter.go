// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

var OpsRampMetricsChannel = make(chan *[]OpsRampMetricsList, 1000)

type opsrampMetricsExporter struct {
	cfg               *Config
	settings          component.TelemetrySettings
	logger            *zap.Logger
	addMetricSuffixes bool
}

// newOpsRampMetricsExporter creates a new exporter for the OpsRamp Metrics format.
func newOpsRampMetricsExporter(cfg *Config, set exporter.Settings) (*opsrampMetricsExporter, error) {
	return &opsrampMetricsExporter{
		cfg:               cfg,
		settings:          set.TelemetrySettings,
		logger:            set.TelemetrySettings.Logger,
		addMetricSuffixes: cfg.AddMetricSuffixes,
	}, nil
}

func (e *opsrampMetricsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushMetricsData takes OTLP metrics data, converts to OpsRampMetric format, and exports it.
func (e *opsrampMetricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	opsrampMetrics, err := e.convertToOpsRampMetrics(md)
	if err != nil {
		e.logger.Error("Failed to convert metrics", zap.Error(err))
		return err
	}

	// Marshal all metrics in a single JSON array
	jsonBytes, err := json.MarshalIndent(opsrampMetrics, "", "  ")
	if err != nil {
		e.logger.Error("Failed to marshal metrics to JSON", zap.Error(err))
		return err
	}

	// Print the JSON representation of all metrics
	//fmt.Printf("OpsRampMetrics JSON:\n%s\n\n", string(jsonBytes))

	// Also log at debug level
	e.logger.Debug("Converted metrics",
		zap.Int("count", len(opsrampMetrics)),
		zap.String("json", string(jsonBytes)))

	select {
	case OpsRampMetricsChannel <- &opsrampMetrics:
		e.logger.Debug("#######OpsRampMetricsExporter: Successfully sent to opsramp metrics channel")
	default:
		e.logger.Error("#######OpsRampMetricsExporter: failed sent to opsramp metrics channel")
	}

	return nil
}

// convertToOpsRampMetrics converts OTLP metrics to our OpsRampMetric format
func (e *opsrampMetricsExporter) convertToOpsRampMetrics(md pmetric.Metrics) ([]OpsRampMetricsList, error) {
	opsrampMetricsList := []OpsRampMetricsList{}

	// Process each resource spans
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		var opsRampMetrics OpsRampMetricsList
		rm := rms.At(i)
		resource := rm.Resource()
		opsRampMetrics.ResourceLabels = attributeMapToLabels(resource.Attributes())

		// Process each instrumentation library spans
		ilms := rm.ScopeMetrics()
		opsrampMetrics := []OpsRampMetric{}

		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricLabels := make(map[string]string)
			// Check if we have a scope (instrumentation library) name
			if ilm.Scope().Name() != "" {
				metricLabels[prometheus.ScopeNameLabelKey] = ilm.Scope().Name()
				if ilm.Scope().Version() != "" {
					metricLabels[prometheus.ScopeVersionLabelKey] = ilm.Scope().Version()
				}
			}

			// Process metrics
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				metricName := metric.Name()

				// Process based on metric type
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					opsrampMetrics = append(opsrampMetrics, e.convertGaugeMetric(metricName, metric.Gauge(), metricLabels)...)
				case pmetric.MetricTypeSum:
					opsrampMetrics = append(opsrampMetrics, e.convertSumMetric(metricName, metric.Sum(), metricLabels)...)
				case pmetric.MetricTypeHistogram:
					opsrampMetrics = append(opsrampMetrics, e.convertHistogramMetric(metricName, metric.Histogram(), metricLabels)...)
				case pmetric.MetricTypeSummary:
					opsrampMetrics = append(opsrampMetrics, e.convertSummaryMetric(metricName, metric.Summary(), metricLabels)...)
				case pmetric.MetricTypeEmpty:
					// Skip empty metrics
				case pmetric.MetricTypeExponentialHistogram:
					// For simplicity, we're not handling exponential histograms in this example
					// You could add support for these if needed
				}
			}
			opsRampMetrics.Metrics = append(opsRampMetrics.Metrics, opsrampMetrics...)
		}
		opsrampMetricsList = append(opsrampMetricsList, opsRampMetrics)
	}

	return opsrampMetricsList, nil
}

// convertGaugeMetric converts gauge metrics to OpsRampMetric format
func (e *opsrampMetricsExporter) convertGaugeMetric(name string, gauge pmetric.Gauge, resourceLabels map[string]string) []OpsRampMetric {
	var metrics []OpsRampMetric

	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		labels := copyLabels(resourceLabels)
		addDataPointAttributes(dp.Attributes(), labels)

		metric := OpsRampMetric{
			MetricName: name,
			Value:      getValueFromDataPoint(dp),
			Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
			Labels:     labels,
		}

		metrics = append(metrics, metric)
	}

	return metrics
}

// convertSumMetric converts sum metrics to OpsRampMetric format
func (e *opsrampMetricsExporter) convertSumMetric(name string, sum pmetric.Sum, resourceLabels map[string]string) []OpsRampMetric {
	var metrics []OpsRampMetric

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		labels := copyLabels(resourceLabels)
		addDataPointAttributes(dp.Attributes(), labels)

		// For cumulative metrics, you might want to add additional metadata
		if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
			labels["temporality"] = "cumulative"
		} else {
			labels["temporality"] = "delta"
		}

		metric := OpsRampMetric{
			MetricName: name,
			Value:      getValueFromDataPoint(dp),
			Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
			Labels:     labels,
		}

		metrics = append(metrics, metric)
	}

	return metrics
}

// convertHistogramMetric converts histogram metrics to OpsRampMetric format
func (e *opsrampMetricsExporter) convertHistogramMetric(name string, histogram pmetric.Histogram, resourceLabels map[string]string) []OpsRampMetric {
	var metrics []OpsRampMetric

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)
		labels := copyLabels(resourceLabels)
		addDataPointAttributes(dp.Attributes(), labels)

		// Add histogram sum
		if e.addMetricSuffixes {
			sumName := name + "_sum"
			metrics = append(metrics, OpsRampMetric{
				MetricName: sumName,
				Value:      dp.Sum(),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     labels,
			})
		} else {
			sumLabels := copyLabels(labels)
			sumLabels["metric_type"] = "histogram_sum"
			metrics = append(metrics, OpsRampMetric{
				MetricName: name,
				Value:      dp.Sum(),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     sumLabels,
			})
		}

		// Add histogram count
		if e.addMetricSuffixes {
			countName := name + "_count"
			metrics = append(metrics, OpsRampMetric{
				MetricName: countName,
				Value:      float64(dp.Count()),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     labels,
			})
		} else {
			countLabels := copyLabels(labels)
			countLabels["metric_type"] = "histogram_count"
			metrics = append(metrics, OpsRampMetric{
				MetricName: name,
				Value:      float64(dp.Count()),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     countLabels,
			})
		}

		// Create bucket metrics for each bucket boundary
		explicitBounds := dp.ExplicitBounds()
		bucketCounts := dp.BucketCounts()

		// In version 0.113.0, these are specific types, not slices
		bucketCountsLen := bucketCounts.Len()

		if bucketCountsLen > 0 {
			for j := 0; j < explicitBounds.Len(); j++ {
				bucketLabels := copyLabels(labels)
				bucketLabels["le"] = fmt.Sprintf("%g", explicitBounds.At(j))

				if e.addMetricSuffixes {
					bucketName := name + "_bucket"
					metrics = append(metrics, OpsRampMetric{
						MetricName: bucketName,
						Value:      float64(bucketCounts.At(j)),
						Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
						Labels:     bucketLabels,
					})
				} else {
					bucketLabels["metric_type"] = "histogram_bucket"
					metrics = append(metrics, OpsRampMetric{
						MetricName: name + "_bucket",
						Value:      float64(bucketCounts.At(j)),
						Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
						Labels:     bucketLabels,
					})
				}
			}

			// Add +Inf bucket
			bucketLabels := copyLabels(labels)
			bucketLabels["le"] = "+Inf"

			if e.addMetricSuffixes {
				bucketName := name + "_bucket"
				metrics = append(metrics, OpsRampMetric{
					MetricName: bucketName,
					Value:      float64(dp.Count()),
					Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
					Labels:     bucketLabels,
				})
			} else {
				bucketLabels["metric_type"] = "histogram_bucket"
				metrics = append(metrics, OpsRampMetric{
					MetricName: name + "_bucket",
					Value:      float64(dp.Count()),
					Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
					Labels:     bucketLabels,
				})
			}
		}
	}

	return metrics
}

// convertSummaryMetric converts summary metrics to OpsRampMetric format
func (e *opsrampMetricsExporter) convertSummaryMetric(name string, summary pmetric.Summary, resourceLabels map[string]string) []OpsRampMetric {
	var metrics []OpsRampMetric

	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)
		labels := copyLabels(resourceLabels)
		addDataPointAttributes(dp.Attributes(), labels)

		// Add summary sum
		if e.addMetricSuffixes {
			sumName := name + "_sum"
			metrics = append(metrics, OpsRampMetric{
				MetricName: sumName,
				Value:      dp.Sum(),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     labels,
			})
		} else {
			sumLabels := copyLabels(labels)
			sumLabels["metric_type"] = "summary_sum"
			metrics = append(metrics, OpsRampMetric{
				MetricName: name,
				Value:      dp.Sum(),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     sumLabels,
			})
		}

		// Add summary count
		if e.addMetricSuffixes {
			countName := name + "_count"
			metrics = append(metrics, OpsRampMetric{
				MetricName: countName,
				Value:      float64(dp.Count()),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     labels,
			})
		} else {
			countLabels := copyLabels(labels)
			countLabels["metric_type"] = "summary_count"
			metrics = append(metrics, OpsRampMetric{
				MetricName: name,
				Value:      float64(dp.Count()),
				Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
				Labels:     countLabels,
			})
		}

		// Create quantile metrics for each quantile value
		qvs := dp.QuantileValues()
		for j := 0; j < qvs.Len(); j++ {
			qv := qvs.At(j)
			quantileLabels := copyLabels(labels)
			quantileLabels["quantile"] = fmt.Sprintf("%g", qv.Quantile())

			if e.addMetricSuffixes {
				// No specific suffix for quantiles in standard Prometheus format
				metrics = append(metrics, OpsRampMetric{
					MetricName: name,
					Value:      qv.Value(),
					Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
					Labels:     quantileLabels,
				})
			} else {
				quantileLabels["metric_type"] = "summary_quantile"
				metrics = append(metrics, OpsRampMetric{
					MetricName: name,
					Value:      qv.Value(),
					Timestamp:  dp.Timestamp().AsTime().UnixMilli(),
					Labels:     quantileLabels,
				})
			}
		}
	}

	return metrics
}

// Helper functions below

func getValueFromDataPoint(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return 0
	}
}

func attributeMapToLabels(attrs pcommon.Map) map[string]string {
	labels := make(map[string]string, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
	return labels
}

func addDataPointAttributes(attrs pcommon.Map, labels map[string]string) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
}

func copyLabels(labels map[string]string) map[string]string {
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[k] = v
	}
	return result
}
