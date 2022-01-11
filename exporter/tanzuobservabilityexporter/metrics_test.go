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

package tanzuobservabilityexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMetricsConsumerNormal(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	sum1 := newMetric("sum1", pdata.MetricDataTypeSum)
	gauge2 := newMetric("gauge2", pdata.MetricDataTypeGauge)
	sum2 := newMetric("sum2", pdata.MetricDataTypeSum)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	mockSumConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeSum}
	sender := &mockFlushCloser{}
	metrics := constructMetrics(gauge1, sum1, gauge2, sum2)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer, mockSumConsumer}, sender, true)

	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	assert.ElementsMatch(t, []string{"gauge1", "gauge2"}, mockGaugeConsumer.names)
	assert.ElementsMatch(t, []string{"sum1", "sum2"}, mockSumConsumer.names)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
	assert.Equal(t, 1, mockSumConsumer.pushInternalMetricsCallCount)
	assert.Equal(t, 1, sender.numFlushCalls)
	assert.Equal(t, 0, sender.numCloseCalls)

	consumer.Close()
	assert.Equal(t, 1, sender.numCloseCalls)
}

func TestMetricsConsumerNone(t *testing.T) {
	consumer := newMetricsConsumer(nil, nil, true)
	metrics := constructMetrics()

	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	consumer.Close()
}

func TestNewMetricsConsumerPanicsWithDuplicateMetricType(t *testing.T) {
	mockGaugeConsumer1 := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	mockGaugeConsumer2 := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}

	assert.Panics(t, func() {
		newMetricsConsumer(
			[]typedMetricConsumer{mockGaugeConsumer1, mockGaugeConsumer2},
			nil,
			true)
	})
}

func TestMetricsConsumerPropagatesErrorsOnFlush(t *testing.T) {
	sender := &mockFlushCloser{errorOnFlush: true}
	metrics := constructMetrics()
	consumer := newMetricsConsumer(nil, sender, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Equal(t, 1, sender.numFlushCalls)
}

func TestMetricsConsumerErrorsWithUnregisteredMetricType(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(nil, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
}

func TestMetricsConsumerErrorConsuming(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{
		typ:            pdata.MetricDataTypeGauge,
		errorOnConsume: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerNoReportingInternalMetrics(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, false)
	assert.NoError(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 0, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerErrorConsumingInternal(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{
		typ: pdata.MetricDataTypeGauge, errorOnPushInternalMetrics: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerRespectContext(t *testing.T) {
	sender := &mockFlushCloser{}
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, sender, true)
	ctx, cancel := context.WithCancel(context.Background())

	cancel()
	assert.Error(t, consumer.Consume(ctx, constructMetrics(gauge1)))

	assert.Zero(t, sender.numFlushCalls)
	assert.Empty(t, mockGaugeConsumer.names)
	assert.Zero(t, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestGaugeConsumerNormal(t *testing.T) {
	verifyGaugeConsumer(t, false)
}

func TestGaugeConsumerErrorSending(t *testing.T) {
	verifyGaugeConsumer(t, true)
}

func TestGaugeConsumerMissingValue(t *testing.T) {
	metric := newMetric("missing.value.metric", pdata.MetricDataTypeGauge)
	dataPoints := metric.Gauge().DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		nil,
		1633123456,
		nil,
		dataPoints,
	)
	// Sending to tanzu observability should fail
	sender := &mockGaugeSender{errorOnSend: true}
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(observedZapCore)
	consumer := newGaugeConsumer(sender, settings)
	var errs []error
	expectedMissingValueCount := 2
	for i := 0; i < expectedMissingValueCount; i++ {
		// This call to Consume does not emit any metrics to tanzuobservability
		// because the metric is missing its value.
		consumer.Consume(metric, &errs)
	}
	assert.Empty(t, errs)

	// This call adds one error to errs because it emits a metric to
	// tanzu observability and emitting there is set up to fail.
	consumer.PushInternalMetrics(&errs)

	// One error from emitting the internal metric
	assert.Len(t, errs, 1)
	assert.Contains(
		t,
		sender.metrics,
		tobsMetric{
			Name:  missingValueMetricName,
			Value: float64(expectedMissingValueCount),
			Tags:  map[string]string{"type": "gauge"},
		},
	)
	allLogs := observedLogs.All()
	assert.Len(t, allLogs, expectedMissingValueCount)
}

func TestSumConsumerDelta(t *testing.T) {
	deltaMetric := newMetric(
		"test.delta.metric", pdata.MetricDataTypeSum)
	sum := deltaMetric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dataPoints := sum.DataPoints()
	dataPoints.EnsureCapacity(2)
	addDataPoint(
		35,
		1635205001,
		map[string]interface{}{
			"env": "dev",
		},
		dataPoints,
	)
	addDataPoint(
		52.375,
		1635205002,
		map[string]interface{}{
			"env": "prod",
		},
		dataPoints,
	)

	sender := &mockSumSender{}
	consumer := newSumConsumer(sender, componenttest.NewNopTelemetrySettings())
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(deltaMetric, &errs)

	expected := []tobsMetric{
		{
			Name:  "test.delta.metric",
			Value: 35.0,
			Tags:  map[string]string{"env": "dev"},
		},
		{
			Name:  "test.delta.metric",
			Value: 52.375,
			Tags:  map[string]string{"env": "prod"},
		},
	}
	assert.ElementsMatch(t, expected, sender.deltaMetrics)
	assert.Empty(t, sender.metrics)
	assert.Empty(t, errs)
}

func TestSumConsumerErrorOnSend(t *testing.T) {
	deltaMetric := newMetric(
		"test.delta.metric", pdata.MetricDataTypeSum)
	sum := deltaMetric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dataPoints := sum.DataPoints()
	dataPoints.EnsureCapacity(2)
	addDataPoint(
		35,
		1635205001,
		map[string]interface{}{
			"env": "dev",
		},
		dataPoints,
	)
	addDataPoint(
		52.375,
		1635205002,
		map[string]interface{}{
			"env": "prod",
		},
		dataPoints,
	)

	sender := &mockSumSender{errorOnSend: true}
	consumer := newSumConsumer(sender, componenttest.NewNopTelemetrySettings())
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(deltaMetric, &errs)

	assert.Len(t, errs, 2)
}

func TestSumConsumerCumulative(t *testing.T) {
	cumulativeMetric := newMetric(
		"test.cumulative.metric", pdata.MetricDataTypeSum)
	sum := cumulativeMetric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dataPoints := sum.DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		62.25,
		1634205001,
		map[string]interface{}{
			"env": "dev",
		},
		dataPoints,
	)
	sender := &mockSumSender{}
	consumer := newSumConsumer(sender, componenttest.NewNopTelemetrySettings())
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// cumulative sums get treated as regular wavefront metrics
	consumer.Consume(cumulativeMetric, &errs)

	expected := []tobsMetric{
		{
			Name:  "test.cumulative.metric",
			Value: 62.25,
			Ts:    1634205001,
			Tags:  map[string]string{"env": "dev"},
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
	assert.Empty(t, sender.deltaMetrics)
	assert.Empty(t, errs)
}

func TestSumConsumerUnspecified(t *testing.T) {
	cumulativeMetric := newMetric(
		"test.unspecified.metric", pdata.MetricDataTypeSum)
	sum := cumulativeMetric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityUnspecified)
	dataPoints := sum.DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		72.25,
		1634206001,
		map[string]interface{}{
			"env": "qa",
		},
		dataPoints,
	)
	sender := &mockSumSender{}
	consumer := newSumConsumer(sender, componenttest.NewNopTelemetrySettings())
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// unspecified sums get treated as regular wavefront metrics
	consumer.Consume(cumulativeMetric, &errs)

	expected := []tobsMetric{
		{
			Name:  "test.unspecified.metric",
			Value: 72.25,
			Ts:    1634206001,
			Tags:  map[string]string{"env": "qa"},
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
	assert.Empty(t, sender.deltaMetrics)
	assert.Empty(t, errs)
}

func TestSumConsumerMissingValue(t *testing.T) {
	metric := newMetric("missing.value.metric", pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
	dataPoints := sum.DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		nil,
		1633123456,
		nil,
		dataPoints,
	)
	sender := &mockSumSender{}
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(observedZapCore)
	consumer := newSumConsumer(sender, settings)
	var errs []error

	expectedMissingValueCount := 2
	for i := 0; i < expectedMissingValueCount; i++ {
		consumer.Consume(metric, &errs)
	}
	consumer.PushInternalMetrics(&errs)

	assert.Len(t, errs, 0)
	assert.Empty(t, sender.deltaMetrics)
	assert.Contains(t, sender.metrics, tobsMetric{
		Name:  missingValueMetricName,
		Value: float64(expectedMissingValueCount),
		Tags:  map[string]string{"type": "sum"},
	})
	allLogs := observedLogs.All()
	assert.Len(t, allLogs, expectedMissingValueCount)
}

// Tests that the histogramConsumer correctly delegates to its
// histogramDataPointConsumers. This tests delta histograms
func TestHistogramConsumerDeltaAggregation(t *testing.T) {
	countAttributeForEachDataPoint := []uint64{2, 5, 10}
	deltaMetric := newHistogramMetricWithDataPoints(
		"delta.metric",
		pdata.MetricAggregationTemporalityDelta,
		countAttributeForEachDataPoint)
	sender := &mockGaugeSender{}
	cumulativeConsumer := &mockHistogramDataPointConsumer{}
	deltaConsumer := &mockHistogramDataPointConsumer{}
	consumer := newHistogramConsumer(
		cumulativeConsumer,
		deltaConsumer,
		sender,
		regularHistogram,
		componenttest.NewNopTelemetrySettings())
	var errs []error
	consumer.Consume(deltaMetric, &errs)

	assert.Empty(t, errs)

	// We had three datapoints. Our mock just captures the metric name of
	// each data point consumed.
	assert.Equal(
		t, []string{"delta.metric", "delta.metric", "delta.metric"}, deltaConsumer.names)
	assert.Equal(t, countAttributeForEachDataPoint, deltaConsumer.counts)
	assert.Empty(t, cumulativeConsumer.names)
	assert.Empty(t, cumulativeConsumer.counts)
}

// Tests that the histogramConsumer correctly delegates to its
// histogramDataPointConsumers. This tests cumulative histograms
func TestHistogramConsumerCumulativeAggregation(t *testing.T) {
	countAttributeForEachDataPoint := []uint64{2, 5, 10}
	cumulativeMetric := newHistogramMetricWithDataPoints(
		"cumulative.metric",
		pdata.MetricAggregationTemporalityCumulative,
		countAttributeForEachDataPoint)
	sender := &mockGaugeSender{}
	cumulativeConsumer := &mockHistogramDataPointConsumer{}
	deltaConsumer := &mockHistogramDataPointConsumer{}
	consumer := newHistogramConsumer(
		cumulativeConsumer,
		deltaConsumer,
		sender,
		regularHistogram,
		componenttest.NewNopTelemetrySettings())
	var errs []error

	consumer.Consume(cumulativeMetric, &errs)

	assert.Empty(t, errs)

	// We had three datapoints. Our mock just captures the metric name of
	// each data point consumed.
	assert.Equal(
		t,
		[]string{"cumulative.metric", "cumulative.metric", "cumulative.metric"},
		cumulativeConsumer.names)
	assert.Equal(t, countAttributeForEachDataPoint, cumulativeConsumer.counts)
	assert.Empty(t, deltaConsumer.names)
	assert.Empty(t, deltaConsumer.counts)
}

// This tests that the histogram consumer correctly counts and logs
// histogram metrics with missing aggregation attribute.
func TestHistogramConsumerNoAggregation(t *testing.T) {

	// Create a histogram metric with missing aggregation attribute
	metric := newHistogramMetricWithDataPoints(
		"missing.aggregation.metric",
		pdata.MetricAggregationTemporalityUnspecified,
		nil)
	sender := &mockGaugeSender{}
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(observedZapCore)
	consumer := newHistogramConsumer(
		&mockHistogramDataPointConsumer{},
		&mockHistogramDataPointConsumer{},
		sender,
		regularHistogram,
		settings,
	)
	assert.Equal(t, pdata.MetricDataTypeHistogram, consumer.Type())
	var errs []error
	expectedNoAggregationCount := 3
	for i := 0; i < expectedNoAggregationCount; i++ {
		consumer.Consume(metric, &errs)
	}
	consumer.PushInternalMetrics(&errs)

	assert.Len(t, errs, 0)
	assert.Contains(t, sender.metrics, tobsMetric{
		Name:  noAggregationTemporalityMetricName,
		Value: float64(expectedNoAggregationCount),
		Tags:  map[string]string{"type": "histogram"},
	})
	allLogs := observedLogs.All()
	assert.Len(t, allLogs, expectedNoAggregationCount)
}

func TestHistogramReporting(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger = zap.New(observedZapCore)
	report := newHistogramReporting(settings)
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	malformedCount := 3
	for i := 0; i < malformedCount; i++ {
		report.LogMalformed(metric)
	}
	noAggregationTemporalityCount := 5
	for i := 0; i < noAggregationTemporalityCount; i++ {
		report.LogNoAggregationTemporality(metric)
	}

	assert.Equal(t, int64(malformedCount), report.Malformed())
	assert.Equal(t, int64(noAggregationTemporalityCount), report.NoAggregationTemporality())
	assert.Equal(
		t,
		malformedCount+noAggregationTemporalityCount,
		observedLogs.Len())

	sender := &mockGaugeSender{}
	var errs []error

	report.Report(sender, &errs)

	assert.Empty(t, errs)
	assert.Contains(
		t,
		sender.metrics,
		tobsMetric{
			Name:  malformedHistogramMetricName,
			Value: float64(malformedCount),
		})
	assert.Contains(
		t,
		sender.metrics,
		tobsMetric{
			Name:  noAggregationTemporalityMetricName,
			Value: float64(noAggregationTemporalityCount),
			Tags:  typeIsHistogramTags,
		})
}

func TestHistogramReportingError(t *testing.T) {
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	sender := &mockGaugeSender{errorOnSend: true}
	var errs []error

	report.Report(sender, &errs)

	assert.NotEmpty(t, errs)
}

func TestCumulativeHistogramDataPointConsumer(t *testing.T) {
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()

	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds([]float64{2.0, 5.0, 10.0})
	histogramDataPoint.SetBucketCounts([]uint64{5, 1, 3, 2})
	setTags(map[string]interface{}{"foo": "bar"}, histogramDataPoint.Attributes())
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Equal(
		t,
		[]tobsMetric{
			{
				Name:  "a.metric",
				Value: 5.0,
				Tags:  map[string]string{"foo": "bar", "le": "2"},
			},
			{
				Name:  "a.metric",
				Value: 6.0,
				Tags:  map[string]string{"foo": "bar", "le": "5"},
			},
			{
				Name:  "a.metric",
				Value: 9.0,
				Tags:  map[string]string{"foo": "bar", "le": "10"},
			},
			{
				Name:  "a.metric",
				Value: 11.0,
				Tags:  map[string]string{"foo": "bar", "le": "+Inf"},
			},
		},
		sender.metrics,
	)
}

func TestCumulativeHistogramDataPointConsumerError(t *testing.T) {
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()

	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds([]float64{2.0, 5.0, 10.0})
	histogramDataPoint.SetBucketCounts([]uint64{5, 1, 3, 2})
	sender := &mockGaugeSender{errorOnSend: true}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	// We tried to send 4 metrics. We get 4 errors.
	assert.Len(t, errs, 4)
}

func TestCumulativeHistogramDataPointConsumerLeInUse(t *testing.T) {
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()
	histogramDataPoint.SetExplicitBounds([]float64{10.0})
	histogramDataPoint.SetBucketCounts([]uint64{4, 12})
	setTags(map[string]interface{}{"le": 8}, histogramDataPoint.Attributes())
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Equal(
		t,
		[]tobsMetric{
			{
				Name:  "a.metric",
				Value: 4.0,
				Tags:  map[string]string{"_le": "8", "le": "10"},
			},
			{
				Name:  "a.metric",
				Value: 16.0,
				Tags:  map[string]string{"_le": "8", "le": "+Inf"},
			},
		},
		sender.metrics,
	)
}

func TestCumulativeHistogramDataPointConsumerMissingBuckets(t *testing.T) {
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Empty(t, sender.metrics)
	assert.Equal(t, int64(1), report.Malformed())
}

func TestDeltaHistogramDataPointConsumer(t *testing.T) {
	metric := newMetric("a.delta.histogram", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()

	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds([]float64{2.0, 5.0, 10.0})
	histogramDataPoint.SetBucketCounts([]uint64{5, 1, 3, 2})
	setDataPointTimestamp(1631234567, histogramDataPoint)
	setTags(
		map[string]interface{}{"bar": "baz"},
		histogramDataPoint.Attributes())
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)

	assert.Equal(
		t,
		[]tobsDistribution{
			{
				Name: "a.delta.histogram",
				Centroids: []histogram.Centroid{
					{Value: 2.0, Count: 5},
					{Value: 3.5, Count: 1},
					{Value: 7.5, Count: 3},
					{Value: 10.0, Count: 2}},
				Granularity: allGranularity,
				Ts:          1631234567,
				Tags:        map[string]string{"bar": "baz"},
			},
		},
		sender.distributions,
	)
	assert.Equal(t, int64(0), report.Malformed())
}

func TestDeltaHistogramDataPointConsumer_OneBucket(t *testing.T) {
	metric := newMetric("one.bucket.delta.histogram", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()

	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds([]float64{})
	histogramDataPoint.SetBucketCounts([]uint64{17})
	setDataPointTimestamp(1641234567, histogramDataPoint)
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)

	assert.Equal(
		t,
		[]tobsDistribution{
			{
				Name:        "one.bucket.delta.histogram",
				Centroids:   []histogram.Centroid{{Value: 0.0, Count: 17}},
				Granularity: allGranularity,
				Ts:          1641234567,
				Tags:        make(map[string]string),
			},
		},
		sender.distributions,
	)
	assert.Equal(t, int64(0), report.Malformed())
}

func TestDeltaHistogramDataPointConsumerError(t *testing.T) {
	metric := newMetric("a.delta.histogram", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()

	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds([]float64{2.0, 5.0, 10.0})
	histogramDataPoint.SetBucketCounts([]uint64{5, 1, 3, 2})
	sender := &mockDistributionSender{errorOnSend: true}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Len(t, errs, 1)
}

func TestDeltaHistogramDataPointConsumerMissingBuckets(t *testing.T) {
	metric := newMetric("a.metric", pdata.MetricDataTypeHistogram)
	histogramDataPoint := pdata.NewHistogramDataPoint()
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(metric, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Empty(t, sender.distributions)
	assert.Equal(t, int64(1), report.Malformed())
}

// Creates a histogram metric with len(countAttributeForEachDataPoint)
// datapoints. name is the name of the histogram metric; temporality
// is the temporality of the histogram metric;
// countAttributeForEachDataPoint contains the count attribute for each
// data point.
func newHistogramMetricWithDataPoints(
	name string,
	temporality pdata.MetricAggregationTemporality,
	countAttributeForEachDataPoint []uint64,
) pdata.Metric {
	result := newMetric(name, pdata.MetricDataTypeHistogram)
	aHistogram := result.Histogram()
	aHistogram.SetAggregationTemporality(temporality)
	aHistogram.DataPoints().EnsureCapacity(len(countAttributeForEachDataPoint))
	for _, count := range countAttributeForEachDataPoint {
		aHistogram.DataPoints().AppendEmpty().SetCount(count)
	}
	return result
}

func verifyGaugeConsumer(t *testing.T, errorOnSend bool) {
	metric := newMetric("test.metric", pdata.MetricDataTypeGauge)
	dataPoints := metric.Gauge().DataPoints()
	dataPoints.EnsureCapacity(2)
	addDataPoint(
		7,
		1631205001,
		map[string]interface{}{"env": "prod", "bucket": 73},
		dataPoints,
	)
	addDataPoint(
		7.5,
		1631205002,
		map[string]interface{}{"env": "prod", "bucket": 73},
		dataPoints,
	)
	expected := []tobsMetric{
		{
			Name:  "test.metric",
			Value: 7.0,
			Ts:    1631205001,
			Tags:  map[string]string{"env": "prod", "bucket": "73"},
		},
		{
			Name:  "test.metric",
			Value: 7.5,
			Ts:    1631205002,
			Tags:  map[string]string{"env": "prod", "bucket": "73"},
		},
	}
	sender := &mockGaugeSender{errorOnSend: errorOnSend}
	consumer := newGaugeConsumer(sender, componenttest.NewNopTelemetrySettings())

	assert.Equal(t, pdata.MetricDataTypeGauge, consumer.Type())
	var errs []error
	consumer.Consume(metric, &errs)
	assert.ElementsMatch(t, expected, sender.metrics)
	if errorOnSend {
		assert.Len(t, errs, len(expected))
	} else {
		assert.Empty(t, errs)
	}
}

func constructMetrics(metricList ...pdata.Metric) pdata.Metrics {
	result := pdata.NewMetrics()
	result.ResourceMetrics().EnsureCapacity(1)
	rm := result.ResourceMetrics().AppendEmpty()
	rm.InstrumentationLibraryMetrics().EnsureCapacity(1)
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.Metrics().EnsureCapacity(len(metricList))
	for _, metric := range metricList {
		metric.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return result
}

func newMetric(name string, typ pdata.MetricDataType) pdata.Metric {
	result := pdata.NewMetric()
	result.SetName(name)
	result.SetDataType(typ)
	return result
}

func addDataPoint(
	value interface{},
	ts int64,
	tags map[string]interface{},
	slice pdata.NumberDataPointSlice,
) {
	dataPoint := slice.AppendEmpty()
	if value != nil {
		setDataPointValue(value, dataPoint)
	}
	setDataPointTimestamp(ts, dataPoint)
	setTags(tags, dataPoint.Attributes())
}

type dataPointWithTimestamp interface {
	SetTimestamp(v pdata.Timestamp)
}

func setDataPointTimestamp(ts int64, dataPoint dataPointWithTimestamp) {
	dataPoint.SetTimestamp(
		pdata.NewTimestampFromTime(time.Unix(ts, 0)))
}

func setDataPointValue(value interface{}, dataPoint pdata.NumberDataPoint) {
	switch v := value.(type) {
	case int:
		dataPoint.SetIntVal(int64(v))
	case int64:
		dataPoint.SetIntVal(v)
	case float64:
		dataPoint.SetDoubleVal(v)
	default:
		panic("Unsupported value type")
	}
}

func setTags(tags map[string]interface{}, attributes pdata.AttributeMap) {
	valueMap := make(map[string]pdata.AttributeValue, len(tags))
	for key, value := range tags {
		switch v := value.(type) {
		case int:
			valueMap[key] = pdata.NewAttributeValueInt(int64(v))
		case int64:
			valueMap[key] = pdata.NewAttributeValueInt(v)
		case float64:
			valueMap[key] = pdata.NewAttributeValueDouble(v)
		case string:
			valueMap[key] = pdata.NewAttributeValueString(v)
		default:
			panic("Invalid value type")
		}
	}
	attributeMap := pdata.NewAttributeMapFromMap(valueMap)
	attributeMap.CopyTo(attributes)
}

type tobsMetric struct {
	Name   string
	Value  float64
	Ts     int64
	Source string
	Tags   map[string]string
}

type mockGaugeSender struct {
	errorOnSend bool
	metrics     []tobsMetric
}

func (m *mockGaugeSender) SendMetric(
	name string, value float64, ts int64, source string, tags map[string]string,
) error {
	m.metrics = append(m.metrics, tobsMetric{
		Name:   name,
		Value:  value,
		Ts:     ts,
		Source: source,
		Tags:   copyTags(tags),
	})
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

type tobsDistribution struct {
	Name        string
	Centroids   []histogram.Centroid
	Granularity map[histogram.Granularity]bool
	Ts          int64
	Source      string
	Tags        map[string]string
}

type mockDistributionSender struct {
	errorOnSend   bool
	distributions []tobsDistribution
}

func (m *mockDistributionSender) SendDistribution(
	name string,
	centroids []histogram.Centroid,
	granularity map[histogram.Granularity]bool,
	ts int64,
	source string,
	tags map[string]string,
) error {
	m.distributions = append(m.distributions, tobsDistribution{
		Name:        name,
		Centroids:   copyCentroids(centroids),
		Granularity: copyGranularity(granularity),
		Ts:          ts,
		Source:      source,
		Tags:        copyTags(tags),
	})
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

type mockTypedMetricConsumer struct {
	typ                          pdata.MetricDataType
	errorOnConsume               bool
	errorOnPushInternalMetrics   bool
	names                        []string
	pushInternalMetricsCallCount int
}

func (m *mockTypedMetricConsumer) Type() pdata.MetricDataType {
	return m.typ
}

func (m *mockTypedMetricConsumer) Consume(metric pdata.Metric, errs *[]error) {
	m.names = append(m.names, metric.Name())
	if m.errorOnConsume {
		*errs = append(*errs, errors.New("error in consume"))
	}
}

func (m *mockTypedMetricConsumer) PushInternalMetrics(errs *[]error) {
	m.pushInternalMetricsCallCount++
	if m.errorOnPushInternalMetrics {
		*errs = append(*errs, errors.New("error in consume"))
	}
}

type mockFlushCloser struct {
	errorOnFlush  bool
	numFlushCalls int
	numCloseCalls int
}

func (m *mockFlushCloser) Flush() error {
	m.numFlushCalls++
	if m.errorOnFlush {
		return errors.New("error flushing")
	}
	return nil
}

func (m *mockFlushCloser) Close() {
	m.numCloseCalls++
}

type mockHistogramDataPointConsumer struct {
	names  []string
	counts []uint64
}

func (m *mockHistogramDataPointConsumer) Consume(
	metric pdata.Metric, h histogramDataPoint, _ *[]error, _ *histogramReporting,
) {
	m.names = append(m.names, metric.Name())
	m.counts = append(m.counts, h.Count())
}

func copyTags(tags map[string]string) map[string]string {
	if tags == nil {
		return nil
	}
	tagsCopy := make(map[string]string, len(tags))
	for k, v := range tags {
		tagsCopy[k] = v
	}
	return tagsCopy
}

type mockSumSender struct {
	errorOnSend  bool
	metrics      []tobsMetric
	deltaMetrics []tobsMetric
}

func (m *mockSumSender) SendMetric(
	name string, value float64, ts int64, source string, tags map[string]string,
) error {
	m.metrics = append(m.metrics, tobsMetric{
		Name:   name,
		Value:  value,
		Ts:     ts,
		Source: source,
		Tags:   copyTags(tags),
	})
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

func (m *mockSumSender) SendDeltaCounter(
	name string, value float64, source string, tags map[string]string,
) error {
	m.deltaMetrics = append(m.deltaMetrics, tobsMetric{
		Name:   name,
		Value:  value,
		Source: source,
		Tags:   copyTags(tags),
	})
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

func copyCentroids(centroids []histogram.Centroid) []histogram.Centroid {
	if centroids == nil {
		return nil
	}
	result := make([]histogram.Centroid, len(centroids))
	copy(result, centroids)
	return result
}

func copyGranularity(
	granularity map[histogram.Granularity]bool) map[histogram.Granularity]bool {
	if granularity == nil {
		return nil
	}
	result := make(map[histogram.Granularity]bool, len(granularity))
	for k, v := range granularity {
		result[k] = v
	}
	return result
}
