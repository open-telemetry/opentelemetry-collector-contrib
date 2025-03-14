// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truereset

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

var (
	tUnknown = timestampFromMs(0)
	t1       = timestampFromMs(1)
	t2       = timestampFromMs(2)
	t3       = timestampFromMs(3)
	t4       = timestampFromMs(4)
	t5       = timestampFromMs(5)

	bounds0  = []float64{1, 2, 4}
	percent0 = []float64{10, 50, 90}

	sum1                  = "sum1"
	sum2                  = "sum2"
	gauge1                = "gauge1"
	histogram1            = "histogram1"
	summary1              = "summary1"
	exponentialHistogram1 = "exponentialHistogram1"

	k1v1k2v2 = []*kv{
		{"k1", "v1"},
		{"k2", "v2"},
	}

	k1v10k2v20 = []*kv{
		{"k1", "v10"},
		{"k2", "v20"},
	}

	k1v100k2v200 = []*kv{
		{"k1", "v100"},
		{"k2", "v200"},
	}

	emptyLabels              []*kv
	k1vEmpty                 = []*kv{{"k1", ""}}
	k1vEmptyk2vEmptyk3vEmpty = []*kv{{"k1", ""}, {"k2", ""}, {"k3", ""}}
)

func TestGauge(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Gauge: round 1 - gauge not adjusted",
			metrics:     metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t1, t1, 44))),
			adjusted:    metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t1, t1, 44))),
		},
		{
			description: "Gauge: round 2 - gauge not adjusted",
			metrics:     metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t2, t2, 66))),
			adjusted:    metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t2, t2, 66))),
		},
		{
			description: "Gauge: round 3 - value less than previous value - gauge is not adjusted",
			metrics:     metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t3, t3, 55))),
			adjusted:    metrics(gaugeMetric(gauge1, doublePoint(k1v1k2v2, t3, t3, 55))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSum(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Sum: round 1 - initial instance, start time is established",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))),
		},
		{
			description: "Sum: round 2 - instance adjusted based on round 1",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t2, t2, 66))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t1, t2, 66))),
		},
		{
			description: "Sum: round 3 - instance reset (value less than previous value), start time is reset",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55))),
		},
		{
			description: "Sum: round 4 - instance adjusted based on round 3",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 72))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t3, t4, 72))),
		},
		{
			description: "Sum: round 5 - instance adjusted based on round 4",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t5, t5, 72))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t3, t5, 72))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSumWithDifferentResources(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Sum: round 1 - initial instance, start time is established",
			metrics:     metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t1, t1, 44)))),
			adjusted:    metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t1, t1, 44)))),
		},
		{
			description: "Sum: round 2 - instance adjusted based on round 1 (metrics in different order)",
			metrics:     metricsFromResourceMetrics(resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t2, t2, 66))), resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t2, t2, 66)))),
			adjusted:    metricsFromResourceMetrics(resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t1, t2, 66))), resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t1, t2, 66)))),
		},
		{
			description: "Sum: round 3 - instance reset (value less than previous value), start time is reset",
			metrics:     metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t3, t3, 55)))),
			adjusted:    metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t3, t3, 55)))),
		},
		{
			description: "Sum: round 4 - instance adjusted based on round 3",
			metrics:     metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 72))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t4, t4, 72)))),
			adjusted:    metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t3, t4, 72))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t3, t4, 72)))),
		},
		{
			description: "Sum: round 5 - instance adjusted based on round 4, sum2 metric resets but sum1 doesn't",
			metrics:     metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t5, t5, 72))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t5, t5, 10)))),
			adjusted:    metricsFromResourceMetrics(resourceMetrics("job1", "instance1", sumMetric(sum1, doublePoint(k1v1k2v2, t3, t5, 72))), resourceMetrics("job2", "instance2", sumMetric(sum2, doublePoint(k1v1k2v2, t5, t5, 10)))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryNoCount(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Summary No Count: round 1 - initial instance, start time is established",
			metrics:     metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
			adjusted:    metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
		},
		{
			description: "Summary No Count: round 2 - instance adjusted based on round 1",
			metrics:     metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t2, t2, 0, 70, percent0, []float64{7, 44, 9}))),
			adjusted:    metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t2, 0, 70, percent0, []float64{7, 44, 9}))),
		},
		{
			description: "Summary No Count: round 3 - instance reset (count less than previous), start time is reset",
			metrics:     metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 0, 66, percent0, []float64{3, 22, 5}))),
			adjusted:    metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 0, 66, percent0, []float64{3, 22, 5}))),
		},
		{
			description: "Summary No Count: round 4 - instance adjusted based on round 3",
			metrics:     metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t4, t4, 0, 96, percent0, []float64{9, 47, 8}))),
			adjusted:    metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t4, 0, 96, percent0, []float64{9, 47, 8}))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryFlagNoRecordedValue(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Summary No Count: round 1 - initial instance, start time is established",
			metrics:     metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
			adjusted:    metrics(summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
		},
		{
			description: "Summary Flag NoRecordedValue: round 2 - instance adjusted based on round 1",
			metrics:     metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, t2, t2))),
			adjusted:    metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummary(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Summary: round 1 - initial instance, start time is established",
			metrics: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			adjusted: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
		},
		{
			description: "Summary: round 2 - instance adjusted based on round 1",
			metrics: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t2, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
			adjusted: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
		},
		{
			description: "Summary: round 3 - instance reset (count less than previous), start time is reset",
			metrics: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
			adjusted: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
		},
		{
			description: "Summary: round 4 - instance adjusted based on round 3",
			metrics: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t4, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
			adjusted: metrics(
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogram(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Histogram: round 1 - initial instance, start time is established",
			metrics:     metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}))),
		}, {
			description: "Histogram: round 2 - instance adjusted based on round 1",
			metrics:     metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{6, 3, 4, 8}))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{6, 3, 4, 8}))),
		}, {
			description: "Histogram: round 3 - instance reset (value less than previous value), start time is reset",
			metrics:     metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7}))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7}))),
		}, {
			description: "Histogram: round 4 - instance adjusted based on round 3",
			metrics:     metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{7, 4, 2, 12}))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t4, bounds0, []uint64{7, 4, 2, 12}))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogramFlagNoRecordedValue(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Histogram: round 1 - initial instance, start time is established",
			metrics:     metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{7, 4, 2, 12}))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{7, 4, 2, 12}))),
		},
		{
			description: "Histogram: round 2 - instance adjusted based on round 1",
			metrics:     metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogramFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Histogram: round 1 - initial instance, start time is unknown",
			metrics:     metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, tUnknown, t1))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			description: "Histogram: round 2 - instance unchanged",
			metrics:     metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(histogramMetric(histogram1, histogramPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

// In TestExponentHistogram we exclude negative buckets on purpose as they are
// not considered the main use case - response times that are most commonly
// observed are never negative. Negative buckets would make the Sum() non
// monotonic and cause unexpected resets.
func TestExponentialHistogram(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Exponential Histogram: round 1 - initial instance, start time is established",
			metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(k1v1k2v2, t1, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
			adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(k1v1k2v2, t1, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
		}, {
			description: "Exponential Histogram: round 2 - instance adjusted based on round 1",
			metrics:     metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(k1v1k2v2, t2, t2, 3, 1, 0, []uint64{}, -2, []uint64{6, 2, 3, 7}))),
			adjusted:    metrics(exponentialHistogramMetric(exponentialHistogram1, exponentialHistogramPoint(k1v1k2v2, t1, t2, 3, 1, 0, []uint64{}, -2, []uint64{6, 2, 3, 7}))),
		}, {
			description: "Exponential Histogram: round 3 - instance reset (value less than previous value), start time is reset",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t3, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t3, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
		}, {
			description: "Exponential Histogram: round 4 - instance adjusted based on round 3",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t4, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t3, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestExponentialHistogramFlagNoRecordedValue(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Histogram: round 1 - initial instance, start time is established",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t1, t1, 0, 2, 2, []uint64{7, 4, 2, 12}, 3, []uint64{}))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPoint(k1v1k2v2, t1, t1, 0, 2, 2, []uint64{7, 4, 2, 12}, 3, []uint64{}))),
		},
		{
			description: "Histogram: round 2 - instance adjusted based on round 1",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestExponentialHistogramFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Histogram: round 1 - initial instance, start time is unknown",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t1))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			description: "Histogram: round 2 - instance unchanged",
			metrics:     metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(exponentialHistogramMetric(histogram1, exponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Summary: round 1 - initial instance, start time is unknown",
			metrics:     metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, tUnknown, t1))),
			adjusted:    metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			description: "Summary: round 2 - instance unchanged",
			metrics:     metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(summaryMetric(summary1, summaryPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestGaugeFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Gauge: round 1 - initial instance, start time is unknown",
			metrics:     metrics(gaugeMetric(gauge1, doublePointNoValue(k1v1k2v2, tUnknown, t1))),
			adjusted:    metrics(gaugeMetric(gauge1, doublePointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			description: "Gauge: round 2 - instance unchanged",
			metrics:     metrics(gaugeMetric(gauge1, doublePointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(gaugeMetric(gauge1, doublePointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSumFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "Sum: round 1 - initial instance, start time is unknown",
			metrics:     metrics(sumMetric("sum1", doublePointNoValue(k1v1k2v2, tUnknown, t1))),
			adjusted:    metrics(sumMetric("sum1", doublePointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			description: "Sum: round 2 - instance unchanged",
			metrics:     metrics(sumMetric("sum1", doublePointNoValue(k1v1k2v2, tUnknown, t2))),
			adjusted:    metrics(sumMetric("sum1", doublePointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestMultiMetrics(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "MultiMetrics: round 1 - combined round 1 of individual metrics",
			metrics: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			adjusted: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
		},
		{
			description: "MultiMetrics: round 2 - combined round 2 of individual metrics",
			metrics: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t2, t2, 66)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t2, t2, 66)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t2, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
			adjusted: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t2, t2, 66)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t2, 66)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{6, 3, 4, 8})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t1, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
		},
		{
			description: "MultiMetrics: round 3 - combined round 3 of individual metrics",
			metrics: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t3, t3, 55)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
			adjusted: metrics(
				gaugeMetric(gauge1, doublePoint(k1v1k2v2, t3, t3, 55)),
				sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 55)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
		},
		{
			description: "MultiMetrics: round 4 - combined round 4 of individual metrics",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 72)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t4, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t3, t4, 72)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t4, bounds0, []uint64{7, 4, 2, 12})),
				summaryMetric(summary1, summaryPoint(k1v1k2v2, t3, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestNewDataPointsAdded(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "New Datapoints: round 1 - two datapoints each",
			metrics: metrics(
				sumMetric(sum1,
					doublePoint(k1v1k2v2, t1, t1, 44),
					doublePoint(k1v100k2v200, t1, t1, 44)),
				histogramMetric(histogram1,
					histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v100k2v200, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1,
					summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v100k2v200, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			adjusted: metrics(
				sumMetric(sum1,
					doublePoint(k1v1k2v2, t1, t1, 44),
					doublePoint(k1v100k2v200, t1, t1, 44)),
				histogramMetric(histogram1,
					histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v100k2v200, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1,
					summaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v100k2v200, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
		},
		{
			description: "New Datapoints: round 2 - new datapoints unchanged, old datapoints adjusted",
			metrics: metrics(
				sumMetric(sum1,
					doublePoint(k1v1k2v2, t2, t2, 44),
					doublePoint(k1v10k2v20, t2, t2, 44),
					doublePoint(k1v100k2v200, t2, t2, 44)),
				histogramMetric(histogram1,
					histogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v10k2v20, t2, t2, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v100k2v200, t2, t2, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1,
					summaryPoint(k1v1k2v2, t2, t2, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v10k2v20, t2, t2, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v100k2v200, t2, t2, 10, 40, percent0, []float64{1, 5, 8})),
			),
			adjusted: metrics(
				sumMetric(sum1,
					doublePoint(k1v1k2v2, t1, t2, 44),
					doublePoint(k1v10k2v20, t2, t2, 44),
					doublePoint(k1v100k2v200, t1, t2, 44)),
				histogramMetric(histogram1,
					histogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v10k2v20, t2, t2, bounds0, []uint64{4, 2, 3, 7}),
					histogramPoint(k1v100k2v200, t1, t2, bounds0, []uint64{4, 2, 3, 7})),
				summaryMetric(summary1,
					summaryPoint(k1v1k2v2, t1, t2, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v10k2v20, t2, t2, 10, 40, percent0, []float64{1, 5, 8}),
					summaryPoint(k1v100k2v200, t1, t2, 10, 40, percent0, []float64{1, 5, 8})),
			),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestMultiTimeseries(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "MultiTimeseries: round 1 - initial first instance, start time is established",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44))),
		},
		{
			description: "MultiTimeseries: round 2 - first instance adjusted based on round 1, initial second instance",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t2, t2, 66)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t2, t2, 20.0)),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t2, 66)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t2, t2, 20.0)),
			),
		},
		{
			description: "MultiTimeseries: round 3 - first instance adjusted based on round 1, second based on round 2",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 88.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t3, t3, 49.0)),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t3, 88.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t2, t3, 49.0)),
			),
		},
		{
			description: "MultiTimeseries: round 4 - first instance reset, second instance adjusted based on round 2, initial third instance",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 87.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t4, t4, 57.0)),
				sumMetric(sum1, doublePoint(k1v100k2v200, t4, t4, 10.0)),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 87.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t2, t4, 57.0)),
				sumMetric(sum1, doublePoint(k1v100k2v200, t4, t4, 10.0)),
			),
		},
		{
			description: "MultiTimeseries: round 5 - first instance adjusted based on round 4, second on round 2, third on round 4",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t5, t5, 90.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t5, t5, 65.0)),
				sumMetric(sum1, doublePoint(k1v100k2v200, t5, t5, 22.0)),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t5, 90.0)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t2, t5, 65.0)),
				sumMetric(sum1, doublePoint(k1v100k2v200, t4, t5, 22.0)),
			),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestEmptyLabels(t *testing.T) {
	script := []*metricsAdjusterTest{
		{
			description: "EmptyLabels: round 1 - initial instance, implicitly empty labels, start time is established",
			metrics:     metrics(sumMetric(sum1, doublePoint(emptyLabels, t1, t1, 44))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(emptyLabels, t1, t1, 44))),
		},
		{
			description: "EmptyLabels: round 2 - instance adjusted based on round 1",
			metrics:     metrics(sumMetric(sum1, doublePoint(emptyLabels, t2, t2, 66))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(emptyLabels, t1, t2, 66))),
		},
		{
			description: "EmptyLabels: round 3 - one explicitly empty label, instance adjusted based on round 1",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1vEmpty, t3, t3, 77))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1vEmpty, t1, t3, 77))),
		},
		{
			description: "EmptyLabels: round 4 - three explicitly empty labels, instance adjusted based on round 1",
			metrics:     metrics(sumMetric(sum1, doublePoint(k1vEmptyk2vEmptyk3vEmpty, t3, t3, 88))),
			adjusted:    metrics(sumMetric(sum1, doublePoint(k1vEmptyk2vEmptyk3vEmpty, t1, t3, 88))),
		},
	}
	runScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestTsGC(t *testing.T) {
	script1 := []*metricsAdjusterTest{
		{
			description: "TsGC: round 1 - initial instances, start time is established",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t1, t1, 20)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t1, t1, 20)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
		},
	}

	script2 := []*metricsAdjusterTest{
		{
			description: "TsGC: round 2 - metrics first timeseries adjusted based on round 2, second timeseries not updated",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t2, t2, 88)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{8, 7, 9, 14})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t2, 88)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{8, 7, 9, 14})),
			),
		},
	}

	script3 := []*metricsAdjusterTest{
		{
			description: "TsGC: round 3 - metrics first timeseries adjusted based on round 2, second timeseries empty due to timeseries gc()",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t3, t3, 99)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t3, t3, 80)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t3, t3, bounds0, []uint64{55, 66, 33, 77})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t3, 99)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t3, t3, 80)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t3, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t3, t3, bounds0, []uint64{55, 66, 33, 77})),
			),
		},
	}

	ma := NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute)

	resourceAttr := "0"
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("0", resourceAttr)
	resourceHash := pdatautil.MapHash(resourceAttrs)

	// run round 1
	runScript(t, ma, script1, resourceAttr)
	// gc the tsmap, unmarking all entries
	tsm, ok := ma.startTimeCache.Get(resourceHash)
	assert.True(t, ok)
	tsm.GC()

	// run round 2 - update metrics first timeseries only
	runScript(t, ma, script2, resourceAttr)
	// gc the tsmap, collecting umarked entries
	tsm, ok = ma.startTimeCache.Get(resourceHash)
	assert.True(t, ok)
	tsm.GC()
	// run round 3 - verify that metrics second timeseries have been gc'd
	runScript(t, ma, script3, resourceAttr)
}

func TestJobGC(t *testing.T) {
	job1Script1 := []*metricsAdjusterTest{
		{
			description: "JobGC: job 1, round 1 - initial instances, adjusted should be empty",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t1, t1, 20)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t1, t1, 44)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t1, t1, 20)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
		},
	}

	job2Script1 := []*metricsAdjusterTest{
		{
			description: "JobGC: job2, round 1 - no metrics adjusted, just trigger gc",
			metrics:     metrics(),
			adjusted:    metrics(),
		},
	}

	job1Script2 := []*metricsAdjusterTest{
		{
			description: "JobGC: job 1, round 2 - metrics timeseries empty due to job-level gc",
			metrics: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 99)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t4, t4, 80)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t4, t4, bounds0, []uint64{55, 66, 33, 77})),
			),
			adjusted: metrics(
				sumMetric(sum1, doublePoint(k1v1k2v2, t4, t4, 99)),
				sumMetric(sum1, doublePoint(k1v10k2v20, t4, t4, 80)),
				histogramMetric(histogram1, histogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{9, 8, 10, 15})),
				histogramMetric(histogram1, histogramPoint(k1v10k2v20, t4, t4, bounds0, []uint64{55, 66, 33, 77})),
			),
		},
	}

	gcInterval := 1 * time.Millisecond
	ma := NewAdjuster(componenttest.NewNopTelemetrySettings(), gcInterval)

	// run job 1, round 1 - all entries marked
	runScript(t, ma, job1Script1, "0")
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// run job 2, round1 - trigger job gc, unmarking all entries
	runScript(t, ma, job2Script1, "1")
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// re-run job 2, round1 - trigger job gc, removing unmarked entries
	runScript(t, ma, job2Script1, "1")
	// ensure that at least one jobsMap.gc() completed
	time.Sleep(gcInterval)
	ma.startTimeCache.MaybeGC()
	time.Sleep(5 * time.Second) // Wait for the goroutine to complete.
	// run job 1, round 2 - verify that all job 1 timeseries have been gc'd
	runScript(t, ma, job1Script2, "0")
}

type metricsAdjusterTest struct {
	description string
	metrics     pmetric.Metrics
	adjusted    pmetric.Metrics
}

func runScript(t *testing.T, ma *Adjuster, tests []*metricsAdjusterTest, additionalResourceAttrs ...string) {
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			adjusted := pmetric.NewMetrics()
			test.metrics.CopyTo(adjusted)
			// Add the instance/job to the input metrics if they aren't already present.
			for i := 0; i < adjusted.ResourceMetrics().Len(); i++ {
				rm := adjusted.ResourceMetrics().At(i)
				for i, attr := range additionalResourceAttrs {
					rm.Resource().Attributes().PutStr(fmt.Sprintf("%d", i), attr)
				}
			}
			var err error
			adjusted, err = ma.AdjustMetrics(context.Background(), adjusted)
			assert.NoError(t, err)

			// Add the instance/job to the expected metrics as well if they aren't already present.
			for i := 0; i < test.adjusted.ResourceMetrics().Len(); i++ {
				rm := test.adjusted.ResourceMetrics().At(i)
				for i, attr := range additionalResourceAttrs {
					rm.Resource().Attributes().PutStr(fmt.Sprintf("%d", i), attr)
				}
			}
			assert.EqualValues(t, test.adjusted, adjusted)
		})
	}
}
