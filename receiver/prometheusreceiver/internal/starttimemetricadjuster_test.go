// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestStartTimeMetricMatch(t *testing.T) {
	const startTime = pcommon.Timestamp(123 * 1e9)
	const currentTime = pcommon.Timestamp(126 * 1e9)
	const matchBuilderStartTime = 124

	tests := []struct {
		name                 string
		inputs               pmetric.Metrics
		startTimeMetricRegex *regexp.Regexp
		expectedStartTime    pcommon.Timestamp
		expectedErr          error
	}{
		{
			name: "regexp_match_sum_metric",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				sumMetric("example_process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				sumMetric("process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
				exponentialHistogramMetric("test_exponential_histogram_metric", exponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(matchBuilderStartTime),
		},
		{
			name: "match_default_sum_start_time_metric",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				sumMetric("example_process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				sumMetric("process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
				exponentialHistogramMetric("test_exponential_histogram_metric", exponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			expectedStartTime: timestampFromFloat64(matchBuilderStartTime + 1),
		},
		{
			name: "regexp_match_gauge_metric",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				gaugeMetric("example_process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				gaugeMetric("process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(matchBuilderStartTime),
		},
		{
			name: "match_default_gauge_start_time_metric",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				gaugeMetric("example_process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
				gaugeMetric("process_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime+1)),
			),
			expectedStartTime: timestampFromFloat64(matchBuilderStartTime + 1),
		},
		{
			name: "empty gauge start time metrics",
			inputs: metrics(
				gaugeMetric("process_start_time_seconds"),
			),
			expectedErr: errNoDataPointsStartTimeMetric,
		},
		{
			name: "empty sum start time metrics",
			inputs: metrics(
				sumMetric("process_start_time_seconds"),
			),
			expectedErr: errNoDataPointsStartTimeMetric,
		},
		{
			name: "unsupported type start time metric",
			inputs: metrics(
				histogramMetric("process_start_time_seconds"),
			),
			expectedErr: errUnsupportedTypeStartTimeMetric,
		},
		{
			name: "regexp_nomatch",
			inputs: metrics(
				sumMetric("subprocess_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.+_process_start_time_seconds$"),
			expectedErr:          errNoStartTimeMetrics,
		},
		{
			name: "nomatch_default_start_time_metric",
			inputs: metrics(
				gaugeMetric("subprocess_start_time_seconds", doublePoint(nil, startTime, currentTime, matchBuilderStartTime)),
			),
			expectedErr: errNoStartTimeMetrics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gcInterval := 10 * time.Millisecond
			stma := NewStartTimeMetricAdjuster(zap.NewNop(), tt.startTimeMetricRegex, gcInterval)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, stma.AdjustMetrics(tt.inputs), tt.expectedErr)
				return
			}

			// We need to make sure the job and instance labels are set before the adjuster is used.
			pmetrics := tt.inputs
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "0")
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "job")
			assert.NoError(t, stma.AdjustMetrics(tt.inputs))
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
		expectedErr          error
	}{
		{
			name: "regexp_match_metric_no_fallback",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
				sumMetric("example_process_start_time_seconds", doublePoint(nil, startTime, currentTime, processStartTimeSeconds)),
				sumMetric("process_start_time_seconds", doublePoint(nil, startTime, currentTime, processStartTimeSeconds)),
				exponentialHistogramMetric("test_exponential_histogram_metric", exponentialHistogramPointSimplified(nil, startTime, currentTime, 3, 1, -5, 3)),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(processStartTimeSeconds),
		},
		{
			name: "regexp_no_regex_match_metric_fallback",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(mockStartTimeSeconds),
		},
		{
			name: "match_no_match_metric_fallback",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
			),
			expectedStartTime: timestampFromFloat64(mockStartTimeSeconds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.SetFeatureGateForTest(t, useCollectorStartTimeFallbackGate, true)
			gcInterval := 10 * time.Millisecond
			stma := NewStartTimeMetricAdjuster(zap.NewNop(), tt.startTimeMetricRegex, gcInterval)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, stma.AdjustMetrics(tt.inputs), tt.expectedErr)
				return
			}

			// To test that the adjuster is using the fallback correctly, override the fallback time to use
			// directly.
			approximateCollectorStartTime = mockStartTime

			// We need to make sure the job and instance labels are set before the adjuster is used.
			pmetrics := tt.inputs
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "0")
			pmetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "job")
			assert.NoError(t, stma.AdjustMetrics(tt.inputs))
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

func TestFallbackAndReset(t *testing.T) {
	mockStartTime := time.Now().Add(-10 * time.Hour).Truncate(time.Second)
	mockTimestamp := pcommon.NewTimestampFromTime(mockStartTime)
	t1 := pcommon.Timestamp(126 * 1e9)
	t2 := pcommon.NewTimestampFromTime(t1.AsTime().Add(1 * time.Hour))
	t3 := pcommon.NewTimestampFromTime(t2.AsTime().Add(1 * time.Hour))
	t4 := pcommon.NewTimestampFromTime(t3.AsTime().Add(1 * time.Hour))
	t5 := pcommon.NewTimestampFromTime(t4.AsTime().Add(1 * time.Hour))
	tests := []struct {
		name        string
		useFallback bool
		scenario    []*metricsAdjusterTest
	}{
		{
			name:        "sum no fallback and reset",
			useFallback: false,
			scenario: []*metricsAdjusterTest{
				{
					description: "Sum: round 1 - initial instance, start time is established",
					metrics: metrics(
						sumMetric("test_sum", doublePoint(nil, t1, t1, 44)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t1, t1, float64(mockTimestamp.AsTime().Unix()))),
					),
					adjusted: metrics(
						sumMetric("test_sum", doublePoint(nil, mockTimestamp, t1, 44)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t1, t1, float64(mockTimestamp.AsTime().Unix()))),
					),
				},
				{
					description: "Sum: round 2 - instance reset (value less than previous value), start time is reset",
					metrics: metrics(
						sumMetric("test_sum", doublePoint(nil, t2, t2, 33)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t2, t2, float64(mockTimestamp.AsTime().Unix()))),
					),
					adjusted: metrics(
						sumMetric("test_sum", doublePoint(nil, t2, t2, 33)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t2, t2, float64(mockTimestamp.AsTime().Unix()))),
					),
				},
				{
					description: "Sum: round 3 - instance adjusted based on round 2",
					metrics: metrics(
						sumMetric("test_sum", doublePoint(nil, t3, t3, 55)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t2, t2, float64(mockTimestamp.AsTime().Unix()))),
					),
					adjusted: metrics(
						sumMetric("test_sum", doublePoint(nil, t2, t3, 55)),
						gaugeMetric("process_start_time_seconds", doublePoint(nil, t2, t2, float64(mockTimestamp.AsTime().Unix()))),
					),
				},
			},
		},
		{
			name:        "sum fallback and reset",
			useFallback: true,
			scenario: []*metricsAdjusterTest{
				{
					description: "Sum: round 1 - initial instance, start time is established",
					metrics:     metrics(sumMetric("test_sum", doublePoint(nil, t1, t1, 44))),
					adjusted:    metrics(sumMetric("test_sum", doublePoint(nil, mockTimestamp, t1, 44))),
				},
				{
					description: "Sum: round 2 - instance adjusted based on round 1",
					metrics:     metrics(sumMetric("test_sum", doublePoint(nil, t2, t2, 66))),
					adjusted:    metrics(sumMetric("test_sum", doublePoint(nil, mockTimestamp, t2, 66))),
				},
				{
					description: "Sum: round 3 - instance reset (value less than previous value), start time is reset",
					metrics:     metrics(sumMetric("test_sum", doublePoint(nil, t3, t3, 55))),
					adjusted:    metrics(sumMetric("test_sum", doublePoint(nil, t3, t3, 55))),
				},
				{
					description: "Sum: round 4 - instance adjusted based on round 3",
					metrics:     metrics(sumMetric("test_sum", doublePoint(nil, t4, t4, 72))),
					adjusted:    metrics(sumMetric("test_sum", doublePoint(nil, t3, t4, 72))),
				},
				{
					description: "Sum: round 5 - instance adjusted based on round 4",
					metrics:     metrics(sumMetric("test_sum", doublePoint(nil, t5, t5, 72))),
					adjusted:    metrics(sumMetric("test_sum", doublePoint(nil, t3, t5, 72))),
				},
			},
		},
		{
			name:        "gauge fallback and reset",
			useFallback: true,
			scenario: []*metricsAdjusterTest{
				{
					description: "Gauge: round 1 - gauge not adjusted",
					metrics:     metrics(gaugeMetric("test_gauge", doublePoint(nil, t1, t1, 44))),
					adjusted:    metrics(gaugeMetric("test_gauge", doublePoint(nil, t1, t1, 44))),
				},
				{
					description: "Gauge: round 2 - gauge not adjusted",
					metrics:     metrics(gaugeMetric("test_gauge", doublePoint(nil, t2, t2, 66))),
					adjusted:    metrics(gaugeMetric("test_gauge", doublePoint(nil, t2, t2, 66))),
				},
				{
					description: "Gauge: round 3 - value less than previous value - gauge is not adjusted",
					metrics:     metrics(gaugeMetric("test_gauge", doublePoint(nil, t3, t3, 55))),
					adjusted:    metrics(gaugeMetric("test_gauge", doublePoint(nil, t3, t3, 55))),
				},
			},
		},
		{
			name:        "histogram fallback and reset",
			useFallback: true,
			scenario: []*metricsAdjusterTest{
				{
					description: "Histogram: round 1 - initial instance, start time is established",
					metrics:     metrics(histogramMetric("test_histogram", histogramPoint(nil, t1, t1, bounds0, []uint64{4, 2, 3, 7}))),
					adjusted:    metrics(histogramMetric("test_histogram", histogramPoint(nil, mockTimestamp, t1, bounds0, []uint64{4, 2, 3, 7}))),
				}, {
					description: "Histogram: round 2 - instance adjusted based on round 1",
					metrics:     metrics(histogramMetric("test_histogram", histogramPoint(nil, t2, t2, bounds0, []uint64{6, 3, 4, 8}))),
					adjusted:    metrics(histogramMetric("test_histogram", histogramPoint(nil, mockTimestamp, t2, bounds0, []uint64{6, 3, 4, 8}))),
				}, {
					description: "Histogram: round 3 - instance reset (value less than previous value), start time is reset",
					metrics:     metrics(histogramMetric("test_histogram", histogramPoint(nil, t3, t3, bounds0, []uint64{5, 3, 2, 7}))),
					adjusted:    metrics(histogramMetric("test_histogram", histogramPoint(nil, t3, t3, bounds0, []uint64{5, 3, 2, 7}))),
				}, {
					description: "Histogram: round 4 - instance adjusted based on round 3",
					metrics:     metrics(histogramMetric("test_histogram", histogramPoint(nil, t4, t4, bounds0, []uint64{7, 4, 2, 12}))),
					adjusted:    metrics(histogramMetric("test_histogram", histogramPoint(nil, t3, t4, bounds0, []uint64{7, 4, 2, 12}))),
				},
			},
		},
		{
			name:        "exponential histogram fallback and reset",
			useFallback: true,
			scenario: []*metricsAdjusterTest{
				{
					description: "Exponential Histogram: round 1 - initial instance, start time is established",
					metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t1, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
					adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, mockTimestamp, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
				},
				{
					description: "Exponential Histogram: round 2 - instance adjusted based on round 1",
					metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t2, t2, 3, 1, 0, []uint64{}, -2, []uint64{6, 2, 3, 7}))),
					adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, mockTimestamp, t2, 3, 1, 0, []uint64{}, -2, []uint64{6, 2, 3, 7}))),
				},
				{
					description: "Exponential Histogram: round 3 - instance reset (value less than previous value), start time is reset",
					metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t3, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
					adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t3, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
				},
				{
					description: "Exponential Histogram: round 4 - instance adjusted based on round 3",
					metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t4, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
					adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(nil, t3, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
				},
			},
		},
		{
			name:        "summary fallback and reset",
			useFallback: true,
			scenario: []*metricsAdjusterTest{
				{
					description: "Summary: round 1 - initial instance, start time is established",
					metrics: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
					),
					adjusted: metrics(
						summaryMetric("test_summary", summaryPoint(nil, mockTimestamp, t1, 10, 40, percent0, []float64{1, 5, 8})),
					),
				},
				{
					description: "Summary: round 2 - instance adjusted based on round 1",
					metrics: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t2, t2, 15, 70, percent0, []float64{7, 44, 9})),
					),
					adjusted: metrics(
						summaryMetric("test_summary", summaryPoint(nil, mockTimestamp, t2, 15, 70, percent0, []float64{7, 44, 9})),
					),
				},
				{
					description: "Summary: round 3 - instance reset (count less than previous), start time is reset",
					metrics: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
					),
					adjusted: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
					),
				},
				{
					description: "Summary: round 4 - instance adjusted based on round 3",
					metrics: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t4, t4, 14, 96, percent0, []float64{9, 47, 8})),
					),
					adjusted: metrics(
						summaryMetric("test_summary", summaryPoint(nil, t3, t4, 14, 96, percent0, []float64{9, 47, 8})),
					),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.SetFeatureGateForTest(t, useCollectorStartTimeFallbackGate, tt.useFallback)
			gcInterval := 10 * time.Millisecond
			stma := NewStartTimeMetricAdjuster(zap.NewNop(), nil, gcInterval)
			// To test that the adjuster is using the fallback correctly, override the fallback time to use
			// directly.
			approximateCollectorStartTime = mockStartTime
			runScript(t, stma, "job", "0", tt.scenario)
		})
	}
}
