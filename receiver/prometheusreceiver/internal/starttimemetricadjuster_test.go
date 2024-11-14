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
	"go.uber.org/zap"
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
			stma := NewStartTimeMetricAdjuster(zap.NewNop(), tt.startTimeMetricRegex, false)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, stma.AdjustMetrics(tt.inputs), tt.expectedErr)
				return
			}
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
			name: "regexp_match_sum_metric_no_fallback",
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
			name: "regexp_match_sum_metric_fallback",
			inputs: metrics(
				sumMetric("test_sum_metric", doublePoint(nil, startTime, currentTime, 16)),
				histogramMetric("test_histogram_metric", histogramPoint(nil, startTime, currentTime, []float64{1, 2}, []uint64{2, 3, 4})),
				summaryMetric("test_summary_metric", summaryPoint(nil, startTime, currentTime, 10, 100, []float64{10, 50, 90}, []float64{9, 15, 48})),
			),
			startTimeMetricRegex: regexp.MustCompile("^.*_process_start_time_seconds$"),
			expectedStartTime:    timestampFromFloat64(mockStartTimeSeconds),
		},
		{
			name: "match_default_sum_start_time_metric_fallback",
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
			stma := NewStartTimeMetricAdjuster(zap.NewNop(), tt.startTimeMetricRegex, true)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, stma.AdjustMetrics(tt.inputs), tt.expectedErr)
				return
			}

			// Make sure the right adjuster is used and one that has the fallback time set.
			metricAdjuster, ok := stma.(*startTimeMetricAdjuster)
			assert.True(t, ok)
			assert.NotNil(t, metricAdjuster.fallbackStartTime)

			// To test that the adjuster is using the fallback correctly, override the fallback time to use
			// directly.
			metricAdjuster.fallbackStartTime = &mockStartTime

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
