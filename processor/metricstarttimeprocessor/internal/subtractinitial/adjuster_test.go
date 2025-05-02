// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subtractinitial // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/testhelper"
)

var (
	tUnknown = testhelper.TimestampFromMs(0)
	t1       = testhelper.TimestampFromMs(1)
	t2       = testhelper.TimestampFromMs(2)
	t3       = testhelper.TimestampFromMs(3)
	t4       = testhelper.TimestampFromMs(4)
	t5       = testhelper.TimestampFromMs(5)

	bounds0  = []float64{1, 2, 4}
	percent0 = []float64{10, 50, 90}

	sum1                  = "sum1"
	sum2                  = "sum2"
	gauge1                = "gauge1"
	histogram1            = "histogram1"
	summary1              = "summary1"
	exponentialHistogram1 = "exponentialHistogram1"

	k1v1k2v2 = []*testhelper.KV{
		{Key: "k1", Value: "v1"},
		{Key: "k2", Value: "v2"},
	}

	k1v10k2v20 = []*testhelper.KV{
		{Key: "k1", Value: "v10"},
		{Key: "k2", Value: "v20"},
	}

	k1v100k2v200 = []*testhelper.KV{
		{Key: "k1", Value: "v100"},
		{Key: "k2", Value: "v200"},
	}

	emptyLabels              []*testhelper.KV
	k1vEmpty                 = []*testhelper.KV{{Key: "k1", Value: ""}}
	k1vEmptyk2vEmptyk3vEmpty = []*testhelper.KV{{Key: "k1", Value: ""}, {Key: "k2", Value: ""}, {Key: "k3", Value: ""}}
)

func TestGauge(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Gauge: round 1 - gauge not adjusted",
			Metrics:     testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))),
			Adjusted:    testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))),
		},
		{
			Description: "Gauge: round 2 - gauge not adjusted",
			Metrics:     testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66))),
			Adjusted:    testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66))),
		},
		{
			Description: "Gauge: round 3 - value less than previous value - gauge is not adjusted",
			Metrics:     testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55))),
			Adjusted:    testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSum(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Sum: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1)),
		},
		{
			Description: "Sum: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22))),
		},
		{
			Description: "Sum: round 3 - instance reset (value less than previous value), start time is reset",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t3, 55))),
		},
		{
			Description: "Sum: round 4 - instance adjusted based on round 3",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t4, t4, 72))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t4, 72))),
		},
		{
			Description: "Sum: round 5 - instance adjusted based on round 4 (value stayed the same))",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t5, t5, 72))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t5, 72))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSumWithDifferentResources(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Sum: round 1 - initial instance, start time is established",
			Metrics:     testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)))),
			Adjusted:    testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1)), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2))),
		},
		{
			Description: "Sum: round 2 - instance adjusted based on round 1 (metrics in different order)",
			Metrics:     testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66))), testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66)))),
			Adjusted:    testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22))), testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22)))),
		},
		{
			Description: "Sum: round 3 - instance reset (value less than previous value), start time is reset",
			Metrics:     testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55)))),
			Adjusted:    testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t3, 55))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t2, t3, 55)))),
		},
		{
			Description: "Sum: round 4 - instance adjusted based on round 3",
			Metrics:     testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t4, t4, 72))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t4, t4, 72)))),
			Adjusted:    testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t4, 72))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t2, t4, 72)))),
		},
		{
			Description: "Sum: round 5 - instance adjusted based on round 4, sum2 metric resets but sum1 doesn't",
			Metrics:     testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t5, t5, 72))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t5, t5, 10)))),
			Adjusted:    testhelper.MetricsFromResourceMetrics(testhelper.ResourceMetrics("job1", "instance1", testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t5, 72))), testhelper.ResourceMetrics("job2", "instance2", testhelper.SumMetric(sum2, testhelper.DoublePoint(k1v1k2v2, t4, t5, 10)))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryNoCount(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Summary No Count: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1)),
		},
		{
			Description: "Summary No Count: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t2, 0, 70, percent0, []float64{7, 44, 9}))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 0, 30, percent0, []float64{7, 44, 9}))),
		},
		{
			Description: "Summary No Count: round 3 - instance reset (count less than previous), start time is reset",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t3, t3, 0, 66, percent0, []float64{3, 22, 5}))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t3, 0, 66, percent0, []float64{3, 22, 5}))),
		},
		{
			Description: "Summary No Count: round 4 - instance adjusted based on round 3",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t4, t4, 0, 96, percent0, []float64{9, 47, 8}))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t4, 0, 96, percent0, []float64{9, 47, 8}))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryFlagNoRecordedValue(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Summary No Count: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t1, 0, 40, percent0, []float64{1, 5, 8}))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1)),
		},
		{
			Description: "Summary Flag NoRecordedValue: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPointNoValue(k1v1k2v2, t2, t2))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummary(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Summary: round 1 - initial instance, start time is established",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1),
			),
		},
		{
			Description: "Summary: round 2 - instance adjusted based on round 1",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 5, 30, percent0, []float64{7, 44, 9})),
			),
		},
		{
			Description: "Summary: round 3 - instance reset (count less than previous), start time is reset",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
		},
		{
			Description: "Summary: round 4 - instance adjusted based on round 3",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t4, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogram(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Histogram: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1)),
		}, {
			Description: "Histogram: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{6, 3, 4, 8}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{2, 1, 1, 1}))),
		}, {
			Description: "Histogram: round 3 - instance reset (value less than previous value), start time is reset",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t3, bounds0, []uint64{5, 3, 2, 7}))),
		}, {
			Description: "Histogram: round 4 - instance adjusted based on round 3",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{7, 4, 2, 12}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t4, bounds0, []uint64{7, 4, 2, 12}))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogramFlagNoRecordedValue(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Histogram: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{7, 4, 2, 12}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1)),
		},
		{
			Description: "Histogram: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestHistogramFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Histogram: round 1 - initial instance, start time is unknown",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPointNoValue(k1v1k2v2, tUnknown, t1))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1)),
		},
		{
			Description: "Histogram: round 2 - instance unchanged",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

// In TestExponentHistogram we exclude negative buckets on purpose as they are
// not considered the main use case - response times that are most commonly
// observed are never negative. Negative buckets would make the Sum() non
// monotonic and cause unexpected resets.
func TestExponentialHistogram(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Exponential Histogram: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1)),
		}, {
			Description: "Exponential Histogram: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t2, t2, 3, 1, 0, []uint64{}, -2, []uint64{6, 2, 3, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t2, 3, 0, 0, []uint64{}, -2, []uint64{2, 0, 0, 0}))),
		}, {
			Description: "Exponential Histogram: round 3 - instance reset (value less than previous value), start time is reset",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t3, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t2, t3, 3, 1, 0, []uint64{}, -2, []uint64{5, 3, 2, 7}))),
		}, {
			Description: "Exponential Histogram: round 4 - instance adjusted based on round 3",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t4, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t2, t4, 3, 1, 0, []uint64{}, -2, []uint64{7, 4, 2, 12}))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestExponentialHistogramFlagNoRecordedValue(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Histogram: round 1 - initial instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t1, 0, 2, 2, []uint64{7, 4, 2, 12}, 3, []uint64{}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1)),
		},
		{
			Description: "Histogram: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPointNoValue(k1v1k2v2, t1, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestExponentialHistogramFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Histogram: round 1 - initial instance, start time is unknown",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t1))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1)),
		},
		{
			Description: "Histogram: round 2 - instance unchanged",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(histogram1, testhelper.ExponentialHistogramPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSummaryFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Summary: round 1 - initial instance, start time is unknown",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPointNoValue(k1v1k2v2, tUnknown, t1))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1)),
		},
		{
			Description: "Summary: round 2 - instance unchanged",
			Metrics:     testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.SummaryMetric(summary1, testhelper.SummaryPointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestGaugeFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Gauge: round 1 - initial instance, start time is unknown",
			Metrics:     testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t1))),
			Adjusted:    testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t1))),
		},
		{
			Description: "Gauge: round 2 - instance unchanged",
			Metrics:     testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.GaugeMetric(gauge1, testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestSumFlagNoRecordedValueFirstObservation(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Sum: round 1 - initial instance, start time is unknown",
			Metrics:     testhelper.Metrics(testhelper.SumMetric("sum1", testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t1))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric("sum1")),
		},
		{
			Description: "Sum: round 2 - instance unchanged",
			Metrics:     testhelper.Metrics(testhelper.SumMetric("sum1", testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t2))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric("sum1", testhelper.DoublePointNoValue(k1v1k2v2, tUnknown, t2))),
		},
	}

	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestMultiMetrics(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "MultiMetrics: round 1 - combined round 1 of individual metrics",
			Metrics: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)),
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1),
				testhelper.SummaryMetric(summary1),
			),
		},
		{
			Description: "MultiMetrics: round 2 - combined round 2 of individual metrics",
			Metrics: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{6, 3, 4, 8})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t2, 15, 70, percent0, []float64{7, 44, 9})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{2, 1, 1, 1})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 5, 30, percent0, []float64{7, 44, 9})),
			),
		},
		{
			Description: "MultiMetrics: round 3 - combined round 3 of individual metrics",
			Metrics: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{5, 3, 2, 7})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t3, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.GaugeMetric(gauge1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 55)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t3, 55)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t3, bounds0, []uint64{5, 3, 2, 7})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t3, 12, 66, percent0, []float64{3, 22, 5})),
			),
		},
		{
			Description: "MultiMetrics: round 4 - combined round 4 of individual metrics",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t4, t4, 72)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{7, 4, 2, 12})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t4, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t4, 72)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t4, bounds0, []uint64{7, 4, 2, 12})),
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t4, 14, 96, percent0, []float64{9, 47, 8})),
			),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestNewDataPointsAdded(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "New Datapoints: round 1 - two datapoints each",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1,
					testhelper.DoublePoint(k1v1k2v2, t1, t1, 44),
					testhelper.DoublePoint(k1v100k2v200, t1, t1, 44)),
				testhelper.HistogramMetric(histogram1,
					testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7}),
					testhelper.HistogramPoint(k1v100k2v200, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				testhelper.SummaryMetric(summary1,
					testhelper.SummaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8}),
					testhelper.SummaryPoint(k1v100k2v200, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1),
				testhelper.SummaryMetric(summary1),
			),
		},
		{
			Description: "New Datapoints: round 2 - new datapoints unchanged, old datapoints adjusted",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1,
					testhelper.DoublePoint(k1v1k2v2, t2, t2, 44),
					testhelper.DoublePoint(k1v10k2v20, t2, t2, 44),
					testhelper.DoublePoint(k1v100k2v200, t2, t2, 44)),
				testhelper.HistogramMetric(histogram1,
					testhelper.HistogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{4, 2, 3, 7}),
					testhelper.HistogramPoint(k1v10k2v20, t2, t2, bounds0, []uint64{4, 2, 3, 7}),
					testhelper.HistogramPoint(k1v100k2v200, t2, t2, bounds0, []uint64{4, 2, 3, 7})),
				testhelper.SummaryMetric(summary1,
					testhelper.SummaryPoint(k1v1k2v2, t2, t2, 10, 40, percent0, []float64{1, 5, 8}),
					testhelper.SummaryPoint(k1v10k2v20, t2, t2, 10, 40, percent0, []float64{1, 5, 8}),
					testhelper.SummaryPoint(k1v100k2v200, t2, t2, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1,
					testhelper.DoublePoint(k1v1k2v2, t1, t2, 0),
					testhelper.DoublePoint(k1v100k2v200, t1, t2, 0)),
				testhelper.HistogramMetric(histogram1,
					testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{0, 0, 0, 0}),
					testhelper.HistogramPoint(k1v100k2v200, t1, t2, bounds0, []uint64{0, 0, 0, 0})),
				testhelper.SummaryMetric(summary1,
					testhelper.SummaryPoint(k1v1k2v2, t1, t2, 0, 0, percent0, []float64{1, 5, 8}),
					testhelper.SummaryPoint(k1v100k2v200, t1, t2, 0, 0, percent0, []float64{1, 5, 8})),
			),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestMultiTimeseries(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "MultiTimeseries: round 1 - initial first instance, start time is established",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1)),
		},
		{
			Description: "MultiTimeseries: round 2 - first instance adjusted based on round 1, initial second instance",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 66)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t2, t2, 20.0)),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22)),
				testhelper.SumMetric(sum1),
			),
		},
		{
			Description: "MultiTimeseries: round 3 - first instance adjusted based on round 1, second based on round 2",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 88.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t3, t3, 49.0)),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t3, 44.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t2, t3, 29.0)),
			),
		},
		{
			Description: "MultiTimeseries: round 4 - first instance reset, second instance adjusted based on round 2, initial third instance",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t4, t4, 87.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t4, t4, 57.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v100k2v200, t4, t4, 10.0)),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t4, 87.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t2, t4, 37.0)),
				testhelper.SumMetric(sum1),
			),
		},
		{
			Description: "MultiTimeseries: round 5 - first instance adjusted based on round 4, second on round 2, third on round 4",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t5, t5, 90.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t5, t5, 65.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v100k2v200, t5, t5, 22.0)),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t5, 90.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t2, t5, 45.0)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v100k2v200, t4, t5, 12.0)),
			),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestEmptyLabels(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "EmptyLabels: round 1 - initial instance, implicitly empty labels, start time is established",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(emptyLabels, t1, t1, 44))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1)),
		},
		{
			Description: "EmptyLabels: round 2 - instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(emptyLabels, t2, t2, 66))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(emptyLabels, t1, t2, 22))),
		},
		{
			Description: "EmptyLabels: round 3 - one explicitly empty label, instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1vEmpty, t3, t3, 77))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1vEmpty, t1, t3, 33))),
		},
		{
			Description: "EmptyLabels: round 4 - three explicitly empty labels, instance adjusted based on round 1",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1vEmptyk2vEmptyk3vEmpty, t3, t3, 88))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1vEmptyk2vEmptyk3vEmpty, t1, t3, 44))),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestDifferentStartTimes(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Sum with created timestamps",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 44))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 44))),
		},
		{
			Description: "Histogram with created timestamps",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{7, 4, 2, 12}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{7, 4, 2, 12}))),
		},
		{
			Description: "Exponential Histogram with created timestamps",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t2, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t2, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
		},
		{
			Description: "Summary with created timestamps",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 10, 40, percent0, []float64{1, 5, 8})),
			),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestResetAfterInitialStart(t *testing.T) {
	script := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Sum round 1: with reference point",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1)),
		},
		{
			Description: "Sum round 2: with reset point",
			Metrics:     testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 22))),
			Adjusted:    testhelper.Metrics(testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 22))),
		},
		{
			Description: "Histogram round 1: with reference point",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{7, 4, 2, 12}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1)),
		},
		{
			Description: "Histogram round 2: with reset point",
			Metrics:     testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{6, 3, 1, 11}))),
			Adjusted:    testhelper.Metrics(testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{6, 3, 1, 11}))),
		},
		{
			Description: "Exponential Histogram round 1: with reference point",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t1, 3, 1, 0, []uint64{}, -2, []uint64{4, 2, 3, 7}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1)),
		},
		{
			Description: "Exponential Histogram round 2: with reset point",
			Metrics:     testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t2, t2, 3, 1, 0, []uint64{}, -2, []uint64{3, 1, 2, 6}))),
			Adjusted:    testhelper.Metrics(testhelper.ExponentialHistogramMetric(exponentialHistogram1, testhelper.ExponentialHistogramPoint(k1v1k2v2, t1, t2, 3, 1, 0, []uint64{}, -2, []uint64{3, 1, 2, 6}))),
		},
		{
			Description: "Summary round 1: with reference point",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t1, 10, 40, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1),
			),
		},
		{
			Description: "Summary round 2: with reset point",
			Metrics: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t2, t2, 1, 4, percent0, []float64{1, 5, 8})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SummaryMetric(summary1, testhelper.SummaryPoint(k1v1k2v2, t1, t2, 1, 4, percent0, []float64{1, 5, 8})),
			),
		},
	}
	testhelper.RunScript(t, NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute), script)
}

func TestTsGC(t *testing.T) {
	script1 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "TsGC: round 1 - initial instances, start time is established",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t1, t1, 20)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1),
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1),
				testhelper.HistogramMetric(histogram1),
			),
		},
	}

	script2 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "TsGC: round 2 - metrics first timeseries adjusted based on round 2, second timeseries not updated",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t2, t2, 88)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t2, t2, bounds0, []uint64{8, 7, 9, 14})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t2, 44)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t2, bounds0, []uint64{4, 5, 6, 7})),
			),
		},
	}

	script3 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "TsGC: round 3 - metrics first timeseries adjusted based on round 2, second timeseries empty due to timeseries gc()",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t3, t3, 99)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t3, t3, 80)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t3, t3, bounds0, []uint64{9, 8, 10, 15})),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v10k2v20, t3, t3, bounds0, []uint64{55, 66, 33, 77})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t3, 55)),
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t3, bounds0, []uint64{5, 6, 7, 8})),
				testhelper.HistogramMetric(histogram1),
			),
		},
	}

	ma := NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute)

	resourceAttr := "0"
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("0", resourceAttr)
	resourceHash := pdatautil.MapHash(resourceAttrs)

	// run round 1
	testhelper.RunScript(t, ma, script1, resourceAttr)
	// gc the tsmap, unmarking all entries
	ma.referenceCache.Get(resourceHash)
	refTsm, ok := ma.referenceCache.Get(resourceHash)
	assert.True(t, ok)
	refTsm.GC()
	// run round 2 - update metrics first timeseries only
	testhelper.RunScript(t, ma, script2, resourceAttr)
	// gc the tsmap, collecting umarked entries
	ma.referenceCache.Get(resourceHash)
	refTsm, ok = ma.referenceCache.Get(resourceHash)
	assert.True(t, ok)
	refTsm.GC()
	// run round 3 - verify that metrics second timeseries have been gc'd
	testhelper.RunScript(t, ma, script3, resourceAttr)
}

func TestJobGC(t *testing.T) {
	job1Script1 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "JobGC: job 1, round 1 - initial instances, adjusted should be empty",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t1, t1, 44)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t1, t1, 20)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t1, t1, bounds0, []uint64{4, 2, 3, 7})),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v10k2v20, t1, t1, bounds0, []uint64{40, 20, 30, 70})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1),
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1),
				testhelper.HistogramMetric(histogram1),
			),
		},
	}

	job2Script1 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "JobGC: job2, round 1 - no metrics adjusted, just trigger gc",
			Metrics:     testhelper.Metrics(),
			Adjusted:    testhelper.Metrics(),
		},
	}

	job1Script2 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "JobGC: job 1, round 2 - metrics timeseries empty due to job-level gc",
			Metrics: testhelper.Metrics(
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v1k2v2, t4, t4, 99)),
				testhelper.SumMetric(sum1, testhelper.DoublePoint(k1v10k2v20, t4, t4, 80)),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v1k2v2, t4, t4, bounds0, []uint64{9, 8, 10, 15})),
				testhelper.HistogramMetric(histogram1, testhelper.HistogramPoint(k1v10k2v20, t4, t4, bounds0, []uint64{55, 66, 33, 77})),
			),
			Adjusted: testhelper.Metrics(
				testhelper.SumMetric(sum1),
				testhelper.SumMetric(sum1),
				testhelper.HistogramMetric(histogram1),
				testhelper.HistogramMetric(histogram1),
			),
		},
	}

	gcInterval := 10 * time.Millisecond
	ma := NewAdjuster(componenttest.NewNopTelemetrySettings(), gcInterval)

	// run job 1, round 1 - all entries marked
	testhelper.RunScript(t, ma, job1Script1, "0")
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// run job 2, round1 - trigger job gc, unmarking all entries
	testhelper.RunScript(t, ma, job2Script1, "1")
	// sleep longer than gcInterval to enable job gc in the next run
	time.Sleep(2 * gcInterval)
	// re-run job 2, round1 - trigger job gc, removing unmarked entries
	testhelper.RunScript(t, ma, job2Script1, "1")
	// ensure that at least one jobsMap.gc() completed
	time.Sleep(gcInterval)
	ma.referenceCache.MaybeGC()
	ma.previousValueCache.MaybeGC()
	time.Sleep(5 * time.Second) // Wait for the goroutine to complete.
	// run job 1, round 2 - verify that all job 1 timeseries have been gc'd
	testhelper.RunScript(t, ma, job1Script2, "0")
}
