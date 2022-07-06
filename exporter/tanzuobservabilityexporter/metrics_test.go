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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestEndToEndGaugeConsumer(t *testing.T) {
	gauge := newMetric("gauge", pmetric.MetricDataTypeGauge)
	dataPoints := gauge.Gauge().DataPoints()
	dataPoints.EnsureCapacity(1)

	// Here we test what happens if the resourcetotelemetry consumer is turned on and is
	// copying resource attributes such as "host.name" into the tags.
	addDataPoint(
		432.25,
		1640123456,
		map[string]interface{}{"source": "renamed", "host.name": "my_source", "env": "prod"},
		dataPoints,
	)
	resourceAttributes := map[string]string{"host.name": "my_source"}
	metrics := constructMetricsWithTags(resourceAttributes, gauge)
	sender := &mockGaugeSender{}
	gaugeConsumer := newGaugeConsumer(sender, componenttest.NewNopTelemetrySettings())
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{gaugeConsumer}, &mockFlushCloser{}, true)
	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	// The "host.name" tag gets filtered out as it contains our source, and the "source"
	// tag gets renamed to "_source"
	assert.Contains(
		t,
		sender.metrics,
		tobsMetric{
			Name:   "gauge",
			Ts:     1640123456,
			Value:  432.25,
			Tags:   map[string]string{"_source": "renamed", "env": "prod"},
			Source: "my_source",
		},
	)

	// Since internal metrics are coming from the exporter itself, we send
	// them with an empty source which defaults to the host name of the
	// exporter.
	assert.Contains(
		t,
		sender.metrics,
		tobsMetric{
			Name:   missingValueMetricName,
			Ts:     0,
			Value:  0.0,
			Tags:   typeIsGaugeTags,
			Source: "",
		},
	)
}

func TestMetricsConsumerNormal(t *testing.T) {
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	sum1 := newMetric("sum1", pmetric.MetricDataTypeSum)
	gauge2 := newMetric("gauge2", pmetric.MetricDataTypeGauge)
	sum2 := newMetric("sum2", pmetric.MetricDataTypeSum)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeGauge}
	mockSumConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeSum}
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

func TestMetricsConsumerNormalWithSourceTag(t *testing.T) {
	sum := newMetric("sum", pmetric.MetricDataTypeSum)
	mockSumConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeSum}
	sender := &mockFlushCloser{}
	tags := map[string]string{"source": "test_source", "test_key": "test_value"}
	metrics := constructMetricsWithTags(tags, sum)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockSumConsumer}, sender, true)
	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	assert.ElementsMatch(t, []string{"sum"}, mockSumConsumer.names)
	assert.ElementsMatch(t, []string{"test_source"}, mockSumConsumer.sources)
	assert.ElementsMatch(t, []string{"source"}, mockSumConsumer.sourceKeys)

	assert.Equal(t, 1, mockSumConsumer.pushInternalMetricsCallCount)
	assert.Equal(t, 1, sender.numFlushCalls)
	assert.Equal(t, 0, sender.numCloseCalls)

	consumer.Close()
	assert.Equal(t, 1, sender.numCloseCalls)
}

func TestMetricsConsumerNormalWithHostnameTag(t *testing.T) {
	sum := newMetric("sum", pmetric.MetricDataTypeSum)
	mockSumConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeSum}
	sender := &mockFlushCloser{}
	tags := map[string]string{"host.name": "test_host.name", "hostname": "test_hostname"}
	metrics := constructMetricsWithTags(tags, sum)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockSumConsumer}, sender, true)

	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	assert.ElementsMatch(t, []string{"sum"}, mockSumConsumer.names)
	assert.ElementsMatch(t, []string{"test_host.name"}, mockSumConsumer.sources)
	assert.ElementsMatch(t, []string{"host.name"}, mockSumConsumer.sourceKeys)

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
	mockGaugeConsumer1 := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeGauge}
	mockGaugeConsumer2 := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeGauge}

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
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(nil, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
}

func TestMetricsConsumerErrorConsuming(t *testing.T) {
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{
		typ:            pmetric.MetricDataTypeGauge,
		errorOnConsume: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerNoReportingInternalMetrics(t *testing.T) {
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeGauge}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, false)
	assert.NoError(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 0, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerErrorConsumingInternal(t *testing.T) {
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{
		typ: pmetric.MetricDataTypeGauge, errorOnPushInternalMetrics: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(
		[]typedMetricConsumer{mockGaugeConsumer}, nil, true)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerRespectContext(t *testing.T) {
	sender := &mockFlushCloser{}
	gauge1 := newMetric("gauge1", pmetric.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pmetric.MetricDataTypeGauge}
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
	metric := newMetric("missing.value.metric", pmetric.MetricDataTypeGauge)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
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
		consumer.Consume(mi, &errs)
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
		"test.delta.metric", pmetric.MetricDataTypeSum)
	sum := deltaMetric.Sum()
	mi := metricInfo{Metric: deltaMetric, Source: "test_source", SourceKey: "host.name"}
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
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
	assert.Equal(t, pmetric.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(mi, &errs)

	expected := []tobsMetric{
		{
			Name:   "test.delta.metric",
			Value:  35.0,
			Source: "test_source",
			Tags:   map[string]string{"env": "dev"},
		},
		{
			Name:   "test.delta.metric",
			Value:  52.375,
			Source: "test_source",
			Tags:   map[string]string{"env": "prod"},
		},
	}
	assert.ElementsMatch(t, expected, sender.deltaMetrics)
	assert.Empty(t, sender.metrics)
	assert.Empty(t, errs)
}

func TestSumConsumerErrorOnSend(t *testing.T) {
	deltaMetric := newMetric(
		"test.delta.metric", pmetric.MetricDataTypeSum)
	sum := deltaMetric.Sum()
	mi := metricInfo{Metric: deltaMetric, Source: "test_source", SourceKey: "host.name"}
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
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
	assert.Equal(t, pmetric.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(mi, &errs)

	assert.Len(t, errs, 2)
}

func TestSumConsumerCumulative(t *testing.T) {
	cumulativeMetric := newMetric(
		"test.cumulative.metric", pmetric.MetricDataTypeSum)
	sum := cumulativeMetric.Sum()
	mi := metricInfo{Metric: cumulativeMetric, Source: "test_source", SourceKey: "host.name"}
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
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
	assert.Equal(t, pmetric.MetricDataTypeSum, consumer.Type())
	var errs []error

	// cumulative sums get treated as regular wavefront metrics
	consumer.Consume(mi, &errs)

	expected := []tobsMetric{
		{
			Name:   "test.cumulative.metric",
			Value:  62.25,
			Ts:     1634205001,
			Source: "test_source",
			Tags:   map[string]string{"env": "dev"},
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
	assert.Empty(t, sender.deltaMetrics)
	assert.Empty(t, errs)
}

func TestSumConsumerUnspecified(t *testing.T) {
	cumulativeMetric := newMetric(
		"test.unspecified.metric", pmetric.MetricDataTypeSum)
	sum := cumulativeMetric.Sum()
	mi := metricInfo{Metric: cumulativeMetric, Source: "test_source", SourceKey: "host.name"}
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityUnspecified)
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
	assert.Equal(t, pmetric.MetricDataTypeSum, consumer.Type())
	var errs []error

	// unspecified sums get treated as regular wavefront metrics
	consumer.Consume(mi, &errs)

	expected := []tobsMetric{
		{
			Name:   "test.unspecified.metric",
			Value:  72.25,
			Ts:     1634206001,
			Source: "test_source",
			Tags:   map[string]string{"env": "qa"},
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
	assert.Empty(t, sender.deltaMetrics)
	assert.Empty(t, errs)
}

func TestSumConsumerMissingValue(t *testing.T) {
	metric := newMetric("missing.value.metric", pmetric.MetricDataTypeSum)
	sum := metric.Sum()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
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
		consumer.Consume(mi, &errs)
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
		pmetric.MetricAggregationTemporalityDelta,
		countAttributeForEachDataPoint)
	mi := metricInfo{Metric: deltaMetric, Source: "test_source", SourceKey: "host.name"}
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
	consumer.Consume(mi, &errs)

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
		pmetric.MetricAggregationTemporalityCumulative,
		countAttributeForEachDataPoint)
	mi := metricInfo{Metric: cumulativeMetric, Source: "test_source", SourceKey: "host.name"}
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

	consumer.Consume(mi, &errs)

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
		pmetric.MetricAggregationTemporalityUnspecified,
		nil)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
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
	assert.Equal(t, pmetric.MetricDataTypeHistogram, consumer.Type())
	var errs []error
	expectedNoAggregationCount := 3
	for i := 0; i < expectedNoAggregationCount; i++ {
		consumer.Consume(mi, &errs)
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
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
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
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{2.0, 5.0, 10.0}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 1, 3, 2}))
	histogramDataPoint.Attributes().UpsertString("foo", "bar")
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Equal(
		t,
		[]tobsMetric{
			{
				Name:   "a.metric",
				Value:  5.0,
				Source: "test_source",
				Tags:   map[string]string{"foo": "bar", "le": "2"},
			},
			{
				Name:   "a.metric",
				Value:  6.0,
				Source: "test_source",
				Tags:   map[string]string{"foo": "bar", "le": "5"},
			},
			{
				Name:   "a.metric",
				Value:  9.0,
				Source: "test_source",
				Tags:   map[string]string{"foo": "bar", "le": "10"},
			},
			{
				Name:   "a.metric",
				Value:  11.0,
				Source: "test_source",
				Tags:   map[string]string{"foo": "bar", "le": "+Inf"},
			},
		},
		sender.metrics,
	)
}

func TestCumulativeHistogramDataPointConsumerError(t *testing.T) {
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{2.0, 5.0, 10.0}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 1, 3, 2}))
	sender := &mockGaugeSender{errorOnSend: true}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	// We tried to send 4 metrics. We get 4 errors.
	assert.Len(t, errs, 4)
}

func TestCumulativeHistogramDataPointConsumerLeInUse(t *testing.T) {
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10.0}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{4, 12}))
	histogramDataPoint.Attributes().UpsertInt("le", 8)
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Equal(
		t,
		[]tobsMetric{
			{
				Name:   "a.metric",
				Value:  4.0,
				Source: "test_source",
				Tags:   map[string]string{"_le": "8", "le": "10"},
			},
			{
				Name:   "a.metric",
				Value:  16.0,
				Source: "test_source",
				Tags:   map[string]string{"_le": "8", "le": "+Inf"},
			},
		},
		sender.metrics,
	)
}

func TestCumulativeHistogramDataPointConsumerMissingBuckets(t *testing.T) {
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	sender := &mockGaugeSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newCumulativeHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Empty(t, sender.metrics)
	assert.Equal(t, int64(1), report.Malformed())
}

func TestDeltaHistogramDataPointConsumer(t *testing.T) {
	metric := newMetric("a.delta.histogram", pmetric.MetricDataTypeHistogram)
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{2.0, 5.0, 10.0}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 1, 3, 2}))
	setDataPointTimestamp(1631234567, histogramDataPoint)
	histogramDataPoint.Attributes().UpsertString("bar", "baz")
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

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
				Source:      "test_source",
				Tags:        map[string]string{"bar": "baz"},
			},
		},
		sender.distributions,
	)
	assert.Equal(t, int64(0), report.Malformed())
}

func TestDeltaHistogramDataPointConsumer_OneBucket(t *testing.T) {
	metric := newMetric("one.bucket.delta.histogram", pmetric.MetricDataTypeHistogram)
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{17}))
	setDataPointTimestamp(1641234567, histogramDataPoint)
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)

	assert.Equal(
		t,
		[]tobsDistribution{
			{
				Name:        "one.bucket.delta.histogram",
				Centroids:   []histogram.Centroid{{Value: 0.0, Count: 17}},
				Granularity: allGranularity,
				Ts:          1641234567,
				Source:      "test_source",
			},
		},
		sender.distributions,
	)
	assert.Equal(t, int64(0), report.Malformed())
}

func TestDeltaHistogramDataPointConsumerError(t *testing.T) {
	metric := newMetric("a.delta.histogram", pmetric.MetricDataTypeHistogram)
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	// Creates bounds of -Inf to <=2.0; >2.0 to <=5.0; >5.0 to <=10.0; >10.0 to +Inf
	histogramDataPoint.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{2.0, 5.0, 10.0}))
	histogramDataPoint.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 1, 3, 2}))
	sender := &mockDistributionSender{errorOnSend: true}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Len(t, errs, 1)
}

func TestDeltaHistogramDataPointConsumerMissingBuckets(t *testing.T) {
	metric := newMetric("a.metric", pmetric.MetricDataTypeHistogram)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
	histogramDataPoint := pmetric.NewHistogramDataPoint()
	sender := &mockDistributionSender{}
	report := newHistogramReporting(componenttest.NewNopTelemetrySettings())
	consumer := newDeltaHistogramDataPointConsumer(sender)
	var errs []error

	consumer.Consume(mi, histogramDataPoint, &errs, report)

	assert.Empty(t, errs)
	assert.Empty(t, sender.distributions)
	assert.Equal(t, int64(1), report.Malformed())
}

func TestSummaries(t *testing.T) {
	summaryMetric := newMetric("test.summary", pmetric.MetricDataTypeSummary)
	summary := summaryMetric.Summary()
	dataPoints := summary.DataPoints()
	dataPoints.EnsureCapacity(2)

	mi := metricInfo{Metric: summaryMetric, Source: "test_source", SourceKey: "host.name"}
	dataPoint := dataPoints.AppendEmpty()
	setQuantileValues(dataPoint, 0.1, 100.0, 0.5, 200.0, 0.9, 300.0, 0.99, 400.0)
	dataPoint.Attributes().UpsertString("foo", "bar")
	dataPoint.SetCount(10)
	dataPoint.SetSum(5000.0)
	setDataPointTimestamp(1645123456, dataPoint)

	dataPoint = dataPoints.AppendEmpty()
	setQuantileValues(dataPoint, 0.2, 75.0, 0.5, 125.0, 0.8, 175.0, 0.95, 225.0)
	dataPoint.Attributes().UpsertString("bar", "baz")
	dataPoint.SetCount(15)
	dataPoint.SetSum(3000.0)
	setDataPointTimestamp(1645123556, dataPoint)

	sender := &mockGaugeSender{}
	consumer := newSummaryConsumer(sender, componenttest.NewNopTelemetrySettings())

	assert.Equal(t, pmetric.MetricDataTypeSummary, consumer.Type())

	var errs []error
	consumer.Consume(mi, &errs)

	assert.Empty(t, errs)

	expected := []tobsMetric{
		{
			Name:   "test.summary",
			Value:  100.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar", "quantile": "0.1"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary",
			Value:  200.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar", "quantile": "0.5"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary",
			Value:  300.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar", "quantile": "0.9"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary",
			Value:  400.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar", "quantile": "0.99"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary_count",
			Value:  10.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary_sum",
			Value:  5000.0,
			Source: "test_source",
			Tags:   map[string]string{"foo": "bar"},
			Ts:     1645123456,
		},
		{
			Name:   "test.summary",
			Value:  75.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz", "quantile": "0.2"},
			Ts:     1645123556,
		},
		{
			Name:   "test.summary",
			Value:  125.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz", "quantile": "0.5"},
			Ts:     1645123556,
		},
		{
			Name:   "test.summary",
			Value:  175.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz", "quantile": "0.8"},
			Ts:     1645123556,
		},
		{
			Name:   "test.summary",
			Value:  225.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz", "quantile": "0.95"},
			Ts:     1645123556,
		},
		{
			Name:   "test.summary_count",
			Value:  15.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz"},
			Ts:     1645123556,
		},
		{
			Name:   "test.summary_sum",
			Value:  3000.0,
			Source: "test_source",
			Tags:   map[string]string{"bar": "baz"},
			Ts:     1645123556,
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
}

func TestSummaries_QuantileTagExists(t *testing.T) {
	summaryMetric := newMetric("test.summary.quantile.tag", pmetric.MetricDataTypeSummary)
	summary := summaryMetric.Summary()
	dataPoints := summary.DataPoints()
	dataPoints.EnsureCapacity(1)

	mi := metricInfo{Metric: summaryMetric, Source: "test_source", SourceKey: "host.name"}
	dataPoint := dataPoints.AppendEmpty()
	setQuantileValues(dataPoint, 0.5, 300.0)
	dataPoint.Attributes().UpsertString("quantile", "exists")
	dataPoint.SetCount(12)
	dataPoint.SetSum(4000.0)
	setDataPointTimestamp(1650123456, dataPoint)

	sender := &mockGaugeSender{}
	consumer := newSummaryConsumer(sender, componenttest.NewNopTelemetrySettings())
	var errs []error
	consumer.Consume(mi, &errs)
	assert.Empty(t, errs)

	expected := []tobsMetric{
		{
			Name:   "test.summary.quantile.tag",
			Value:  300.0,
			Source: "test_source",
			Tags:   map[string]string{"_quantile": "exists", "quantile": "0.5"},
			Ts:     1650123456,
		},
		{
			Name:   "test.summary.quantile.tag_count",
			Value:  12.0,
			Source: "test_source",
			Tags:   map[string]string{"_quantile": "exists"},
			Ts:     1650123456,
		},
		{
			Name:   "test.summary.quantile.tag_sum",
			Value:  4000.0,
			Source: "test_source",
			Tags:   map[string]string{"_quantile": "exists"},
			Ts:     1650123456,
		},
	}
	assert.ElementsMatch(t, expected, sender.metrics)
}

func TestSummariesConsumer_ErrorSending(t *testing.T) {
	summaryMetric := newMetric("test.summary.error", pmetric.MetricDataTypeSummary)
	summary := summaryMetric.Summary()
	mi := metricInfo{Metric: summaryMetric, Source: "test_source", SourceKey: "host.name"}
	dataPoints := summary.DataPoints()
	dataPoints.EnsureCapacity(1)

	dataPoint := dataPoints.AppendEmpty()
	dataPoint.SetCount(13)
	dataPoint.SetSum(3900.0)

	sender := &mockGaugeSender{errorOnSend: true}
	consumer := newSummaryConsumer(sender, componenttest.NewNopTelemetrySettings())
	var errs []error
	consumer.Consume(mi, &errs)
	assert.NotEmpty(t, errs)
}

// Sets quantile values for a summary data point
func setQuantileValues(dataPoint pmetric.SummaryDataPoint, quantileValues ...float64) {
	if len(quantileValues)%2 != 0 {
		panic("quantileValues must be quantile, value, quantile, value, ...")
	}
	length := len(quantileValues) / 2
	quantileValuesSlice := dataPoint.QuantileValues()
	quantileValuesSlice.EnsureCapacity(length)
	for i := 0; i < length; i++ {
		quantileValueObj := quantileValuesSlice.AppendEmpty()
		quantileValueObj.SetQuantile(quantileValues[2*i])
		quantileValueObj.SetValue(quantileValues[2*i+1])
	}
}

func TestExponentialHistogramConsumerSpec(t *testing.T) {
	metric := newExponentialHistogramMetricWithDataPoints(
		"a.metric", pmetric.MetricAggregationTemporalityDelta, []uint64{4, 7, 11})
	assert.Equal(t, pmetric.MetricDataTypeExponentialHistogram, exponentialHistogram.Type())
	aHistogram := exponentialHistogram.AsHistogram(metric)
	assert.Equal(t, pmetric.MetricAggregationTemporalityDelta, aHistogram.AggregationTemporality())
	assert.Equal(t, 3, aHistogram.Len())
	assert.Equal(t, uint64(4), aHistogram.At(0).Count())
	assert.Equal(t, uint64(7), aHistogram.At(1).Count())
	assert.Equal(t, uint64(11), aHistogram.At(2).Count())
}

func TestExponentialHistogramDataPoint(t *testing.T) {
	dataPoint := pmetric.NewExponentialHistogramDataPoint()
	dataPoint.SetScale(1)
	dataPoint.Negative().SetOffset(6)
	dataPoint.Negative().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{15, 16, 17}))
	dataPoint.Positive().SetOffset(3)
	dataPoint.Positive().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 6, 7, 8}))
	dataPoint.SetZeroCount(2)
	dataPoint.Attributes().UpsertString("foo", "bar")
	dataPoint.Attributes().UpsertString("baz", "7")
	setDataPointTimestamp(1640198765, dataPoint)
	h := newExponentialHistogramDataPoint(dataPoint)
	assert.Equal(t, []uint64{17, 16, 15, 2, 5, 6, 7, 8, 0}, h.BucketCounts().AsRaw())
	assert.InDeltaSlice(
		t,
		[]float64{-16.0, -11.3137, -8.0, 2.8284, 4.0, 5.6569, 8.0, 11.3137},
		h.ExplicitBounds().AsRaw(),
		0.0001)
	assert.Equal(t, map[string]string{"foo": "bar", "baz": "7"}, attributesToTags(h.Attributes()))
	assert.Equal(t, int64(1640198765), h.Timestamp().AsTime().Unix())
}

func TestExponentialHistogramDataPoint_ZeroOnly(t *testing.T) {
	dataPoint := pmetric.NewExponentialHistogramDataPoint()
	dataPoint.SetScale(0)
	dataPoint.Negative().SetOffset(2)
	dataPoint.Positive().SetOffset(1)
	dataPoint.SetZeroCount(5)
	h := newExponentialHistogramDataPoint(dataPoint)
	assert.Equal(t, []uint64{5, 0}, h.BucketCounts().AsRaw())
	assert.InDeltaSlice(t, []float64{2.0}, h.ExplicitBounds().AsRaw(), 0.0001)
}

func TestAttributesToTagsForMetrics(t *testing.T) {
	// 1. Empty sourceKey: does not change resulting wfTags
	attrMap := newMap(map[string]string{"k": "v"})
	wfTags := attributesToTagsForMetrics(attrMap, "")
	assert.Equal(t, map[string]string{"k": "v"}, wfTags)

	// 2. sourceKey exists in attrMap: delete from resulting wfTags
	attrMap = newMap(map[string]string{"k": "v", "a_key": "a_val"})
	wfTags = attributesToTagsForMetrics(attrMap, "a_key")
	assert.Equal(t, map[string]string{"k": "v"}, wfTags)

	// 3. sourceKey is "source": delete from resulting wfTags. This scenario should only occur when
	//    the Resource Attrs contained an Attr named "source", which is the determinant of sourceKey.
	attrMap = newMap(map[string]string{"k": "v", "source": "a_source"})
	wfTags = attributesToTagsForMetrics(attrMap, "source")
	assert.Equal(t, map[string]string{"k": "v"}, wfTags)

	// 4. sourceKey is not "source", but a "source" tag exists in Attrs
	//    This scenario should only occur if Resource Attrs did not include a "source" attr, but
	//    the Attrs at the metric-level happened to include an attr named "source". In this edge-
	//    case scenario, rename the resulting wfTag to "_source" so the data isn't lost.
	attrMap = newMap(map[string]string{"k": "v", "source": "a_val"})
	wfTags = attributesToTagsForMetrics(attrMap, "")
	assert.Equal(t, map[string]string{"k": "v", "_source": "a_val"}, wfTags)
}

// Creates a histogram metric with len(countAttributeForEachDataPoint)
// datapoints. name is the name of the histogram metric; temporality
// is the temporality of the histogram metric;
// countAttributeForEachDataPoint contains the count attribute for each
// data point.
func newHistogramMetricWithDataPoints(
	name string,
	temporality pmetric.MetricAggregationTemporality,
	countAttributeForEachDataPoint []uint64,
) pmetric.Metric {
	result := newMetric(name, pmetric.MetricDataTypeHistogram)
	aHistogram := result.Histogram()
	aHistogram.SetAggregationTemporality(temporality)
	aHistogram.DataPoints().EnsureCapacity(len(countAttributeForEachDataPoint))
	for _, count := range countAttributeForEachDataPoint {
		aHistogram.DataPoints().AppendEmpty().SetCount(count)
	}
	return result
}

// Works like newHistogramMetricWithDataPoints but creates an exponential histogram metric
func newExponentialHistogramMetricWithDataPoints(
	name string,
	temporality pmetric.MetricAggregationTemporality,
	countAttributeForEachDataPoint []uint64,
) pmetric.Metric {
	result := newMetric(name, pmetric.MetricDataTypeExponentialHistogram)
	aHistogram := result.ExponentialHistogram()
	aHistogram.SetAggregationTemporality(temporality)
	aHistogram.DataPoints().EnsureCapacity(len(countAttributeForEachDataPoint))
	for _, count := range countAttributeForEachDataPoint {
		aHistogram.DataPoints().AppendEmpty().SetCount(count)
	}
	return result
}

func verifyGaugeConsumer(t *testing.T, errorOnSend bool) {
	metric := newMetric("test.metric", pmetric.MetricDataTypeGauge)
	mi := metricInfo{Metric: metric, Source: "test_source", SourceKey: "host.name"}
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
			Name:   "test.metric",
			Value:  7.0,
			Ts:     1631205001,
			Source: "test_source",
			Tags:   map[string]string{"env": "prod", "bucket": "73"},
		},
		{
			Name:   "test.metric",
			Value:  7.5,
			Ts:     1631205002,
			Source: "test_source",
			Tags:   map[string]string{"env": "prod", "bucket": "73"},
		},
	}
	sender := &mockGaugeSender{errorOnSend: errorOnSend}
	consumer := newGaugeConsumer(sender, componenttest.NewNopTelemetrySettings())

	assert.Equal(t, pmetric.MetricDataTypeGauge, consumer.Type())
	var errs []error
	consumer.Consume(mi, &errs)
	assert.ElementsMatch(t, expected, sender.metrics)
	if errorOnSend {
		assert.Len(t, errs, len(expected))
	} else {
		assert.Empty(t, errs)
	}
}

func constructMetrics(metricList ...pmetric.Metric) pmetric.Metrics {
	result := pmetric.NewMetrics()
	result.ResourceMetrics().EnsureCapacity(1)
	rm := result.ResourceMetrics().AppendEmpty()
	rm.ScopeMetrics().EnsureCapacity(1)
	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Metrics().EnsureCapacity(len(metricList))
	for _, metric := range metricList {
		metric.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return result
}

func constructMetricsWithTags(tags map[string]string, metricList ...pmetric.Metric) pmetric.Metrics {
	result := pmetric.NewMetrics()
	result.ResourceMetrics().EnsureCapacity(1)
	rm := result.ResourceMetrics().AppendEmpty()
	for key, val := range tags {
		rm.Resource().Attributes().InsertString(key, val)
	}
	rm.ScopeMetrics().EnsureCapacity(1)
	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Metrics().EnsureCapacity(len(metricList))
	for _, metric := range metricList {
		metric.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return result
}

func newMetric(name string, typ pmetric.MetricDataType) pmetric.Metric {
	result := pmetric.NewMetric()
	result.SetName(name)
	result.SetDataType(typ)
	return result
}

func addDataPoint(
	value interface{},
	ts int64,
	tags map[string]interface{},
	slice pmetric.NumberDataPointSlice,
) {
	dataPoint := slice.AppendEmpty()
	if value != nil {
		setDataPointValue(value, dataPoint)
	}
	setDataPointTimestamp(ts, dataPoint)
	pcommon.NewMapFromRaw(tags).CopyTo(dataPoint.Attributes())
}

type dataPointWithTimestamp interface {
	SetTimestamp(v pcommon.Timestamp)
}

func setDataPointTimestamp(ts int64, dataPoint dataPointWithTimestamp) {
	dataPoint.SetTimestamp(
		pcommon.NewTimestampFromTime(time.Unix(ts, 0)))
}

func setDataPointValue(value interface{}, dataPoint pmetric.NumberDataPoint) {
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
	typ                          pmetric.MetricDataType
	errorOnConsume               bool
	errorOnPushInternalMetrics   bool
	names                        []string
	sources                      []string
	sourceKeys                   []string
	pushInternalMetricsCallCount int
}

func (m *mockTypedMetricConsumer) Type() pmetric.MetricDataType {
	return m.typ
}

func (m *mockTypedMetricConsumer) Consume(mi metricInfo, errs *[]error) {
	m.names = append(m.names, mi.Name())
	m.sources = append(m.sources, mi.Source)
	m.sourceKeys = append(m.sourceKeys, mi.SourceKey)
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

func (m *mockHistogramDataPointConsumer) Consume(mi metricInfo, histogram histogramDataPoint, errs *[]error, reporting *histogramReporting) {
	m.names = append(m.names, mi.Name())
	m.counts = append(m.counts, histogram.Count())
}

func copyTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
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
