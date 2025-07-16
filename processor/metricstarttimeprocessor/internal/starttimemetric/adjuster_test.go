// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starttimemetric

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/testhelper"
)

func TestStartTimeMetricMatch(t *testing.T) {
	const startTime = pcommon.Timestamp(123 * 1e9)
	const currentTime = pcommon.Timestamp(126 * 1e9)
	const collectorStartTime = pcommon.Timestamp(129 * 1e9)
	const matchBuilderStartTime = 124

	tests := []struct {
		name                 string
		inputs               pmetric.Metrics
		startTimeMetricRegex *regexp.Regexp
		expectedStartTime    pcommon.Timestamp
	}{
		{
			name: "regexp_match_sum_metric",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				testhelper.SumMetric("example_process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				testhelper.SumMetric("process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
				testhelper.ExponentialHistogramMetric("test_exponential_histogram_metric", testhelper.ExponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(matchBuilderStartTime),
		},
		{
			name: "match_default_sum_start_time_metric",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				testhelper.SumMetric("example_process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				testhelper.SumMetric("process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
				testhelper.ExponentialHistogramMetric("test_exponential_histogram_metric", testhelper.ExponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			expectedStartTime: timestampFromFloat64(matchBuilderStartTime + 1),
		},
		{
			name: "regexp_match_gauge_metric",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				testhelper.GaugeMetric("example_process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				testhelper.GaugeMetric("process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(matchBuilderStartTime),
		},
		{
			name: "match_default_gauge_start_time_metric",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				testhelper.GaugeMetric("example_process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				testhelper.GaugeMetric("process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
			),
			expectedStartTime: timestampFromFloat64(matchBuilderStartTime + 1),
		},
		{
			name: "empty gauge start time metrics",
			inputs: testhelper.Metrics(
				testhelper.GaugeMetric("process_start_time_seconds"),
			),
			expectedStartTime: collectorStartTime,
		},
		{
			name: "empty sum start time metrics",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("process_start_time_seconds"),
			),
			expectedStartTime: collectorStartTime,
		},
		{
			name: "unsupported type start time metric",
			inputs: testhelper.Metrics(
				testhelper.HistogramMetric("process_start_time_seconds"),
			),
			expectedStartTime: collectorStartTime,
		},
		{
			name: "regexp_nomatch",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("subprocess_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.+_process_start_time_seconds$"),
			expectedStartTime:    collectorStartTime,
		},
		{
			name: "nomatch_default_start_time_metric",
			inputs: testhelper.Metrics(
				testhelper.GaugeMetric("subprocess_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
			),
			expectedStartTime: collectorStartTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// To test that the adjuster is using the fallback correctly, override the fallback time to use
			// directly.
			approximateCollectorStartTime = collectorStartTime.AsTime()

			stma := NewAdjuster(componenttest.NewNopTelemetrySettings(), tt.startTimeMetricRegex)

			// We need to make sure the job and instance labels are set before the adjuster is used.
			pmetrics := tt.inputs
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "0")
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "job")
			_, err := stma.AdjustMetrics(context.Background(), tt.inputs)
			assert.NoError(t, err)
			for i := 0; i < tt.inputs.ResourceMetrics().Len(); i++ {
				rm := tt.inputs.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					ilm := rm.ScopeMetrics().At(j)
					for k := 0; k < ilm.Metrics().Len(); k++ {
						metric := ilm.Metrics().At(k)
						switch metric.Type() {
						case pmetric.MetricTypeSum:
							dps := metric.Sum().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeSummary:
							dps := metric.Summary().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeHistogram:
							dps := metric.Histogram().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeExponentialHistogram:
							dps := metric.ExponentialHistogram().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge:
						}
					}
				}
			}
		})
	}
}

func TestStartTimeMetricFallback(t *testing.T) {
	const startTime = pcommon.Timestamp(123 * 1e9)
	const currentTime = pcommon.Timestamp(126 * 1e9)
	mockStartTime := time.Now().Add(-10 * time.Hour)
	mockStartTimeSeconds := float64(mockStartTime.Unix())
	processStartTime := mockStartTime.Add(-10 * time.Hour)
	processStartTimeSeconds := float64(processStartTime.Unix())

	tests := []struct {
		name                 string
		inputs               pmetric.Metrics
		startTimeMetricRegex *regexp.Regexp
		expectedStartTime    pcommon.Timestamp
	}{
		{
			name: "regexp_match_metric_no_fallback",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				testhelper.SumMetric("example_process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, processStartTimeSeconds)),
				testhelper.SumMetric("process_start_time_seconds", testhelper.DoublePoint(nil, startTime, currentTime, processStartTimeSeconds)),
				testhelper.ExponentialHistogramMetric("test_exponential_histogram_metric", testhelper.ExponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(processStartTimeSeconds),
		},
		{
			name: "regexp_no_regex_match_metric_fallback",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(mockStartTimeSeconds),
		},
		{
			name: "match_no_match_metric_fallback",
			inputs: testhelper.Metrics(
				testhelper.SumMetric("test_sum_metric", testhelper.DoublePoint(nil, startTime, currentTime, 16)),
				testhelper.HistogramMetric("test_histogram_metric", testhelper.HistogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				testhelper.SummaryMetric("test_summary_metric", testhelper.SummaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
			),
			expectedStartTime: timestampFromFloat64(mockStartTimeSeconds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stma := NewAdjuster(componenttest.NewNopTelemetrySettings(), tt.startTimeMetricRegex)

			// To test that the adjuster is using the fallback correctly, override the fallback time to use
			// directly.
			approximateCollectorStartTime = mockStartTime

			// We need to make sure the job and instance labels are set before the adjuster is used.
			pmetrics := tt.inputs
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "0")
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "job")
			_, err := stma.AdjustMetrics(context.Background(), tt.inputs)
			assert.NoError(t, err, tt.inputs)
			for i := 0; i < tt.inputs.ResourceMetrics().Len(); i++ {
				rm := tt.inputs.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					ilm := rm.ScopeMetrics().At(j)
					for k := 0; k < ilm.Metrics().Len(); k++ {
						metric := ilm.Metrics().At(k)
						switch metric.Type() {
						case pmetric.MetricTypeSum:
							dps := metric.Sum().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeSummary:
							dps := metric.Summary().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						case pmetric.MetricTypeHistogram:
							dps := metric.Histogram().DataPoints()
							for l := 0; l < dps.Len(); l++ {
								assert.Equal(t, tt.expectedStartTime, dps.At(l).StartTimestamp())
							}
						}
					}
				}
			}
		})
	}
}
