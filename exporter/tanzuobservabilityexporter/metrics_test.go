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
	"github.com/stretchr/testify/require"
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
	consumer := newMetricsConsumer([]typedMetricConsumer{mockGaugeConsumer, mockSumConsumer}, sender)

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
	consumer := newMetricsConsumer(nil, nil)
	metrics := constructMetrics()

	assert.NoError(t, consumer.Consume(context.Background(), metrics))

	consumer.Close()
}

func TestNewMetricsConsumerPanicsWithDuplicateMetricType(t *testing.T) {
	mockGaugeConsumer1 := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	mockGaugeConsumer2 := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}

	assert.Panics(t, func() {
		newMetricsConsumer([]typedMetricConsumer{mockGaugeConsumer1, mockGaugeConsumer2}, nil)
	})
}

func TestMetricsConsumerPropagatesErrorsOnFlush(t *testing.T) {
	sender := &mockFlushCloser{errorOnFlush: true}
	metrics := constructMetrics()
	consumer := newMetricsConsumer(nil, sender)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Equal(t, 1, sender.numFlushCalls)
}

func TestMetricsConsumerErrorsWithUnregisteredMetricType(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(nil, nil)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
}

func TestMetricsConsumerErrorConsuming(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge, errorOnConsume: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer([]typedMetricConsumer{mockGaugeConsumer}, nil)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerErrorConsumingInternal(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{
		typ: pdata.MetricDataTypeGauge, errorOnPushInternalMetrics: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer([]typedMetricConsumer{mockGaugeConsumer}, nil)

	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
	assert.Equal(t, 1, mockGaugeConsumer.pushInternalMetricsCallCount)
}

func TestMetricsConsumerRespectContext(t *testing.T) {
	sender := &mockFlushCloser{}
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockTypedMetricConsumer{typ: pdata.MetricDataTypeGauge}
	consumer := newMetricsConsumer([]typedMetricConsumer{mockGaugeConsumer}, sender)
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

func TestGaugeConsumerMissingValueNoLogging(t *testing.T) {
	metric := newMetric("bad.metric", pdata.MetricDataTypeGauge)
	dataPoints := metric.Gauge().DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		nil,
		1633123456,
		nil,
		dataPoints,
	)
	sender := &mockGaugeSender{}
	consumer := newGaugeConsumer(sender, nil)
	var errs []error

	consumer.Consume(metric, &errs)
	consumer.PushInternalMetrics(&errs)

	assert.Empty(t, errs)
	assert.Empty(t, sender.metrics)
}

func TestGaugeConsumerMissingValue(t *testing.T) {
	metric := newMetric("bad.metric", pdata.MetricDataTypeGauge)
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
	consumer := newGaugeConsumer(sender, &consumerOptions{
		Logger:                zap.New(observedZapCore),
		ReportInternalMetrics: true,
	})
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
	// Only the internal metric was sent
	require.Len(t, sender.metrics, 1)
	assert.Equal(t, tobsMetric{
		Name:  missingValueMetricName,
		Value: float64(expectedMissingValueCount),
		Tags:  map[string]string{"type": "gauge"}},
		sender.metrics[0])
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
	consumer := newSumConsumer(sender, nil)
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(deltaMetric, &errs)
	consumer.PushInternalMetrics(&errs)

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
	consumer := newSumConsumer(sender, nil)
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// delta sums get treated as delta counters
	consumer.Consume(deltaMetric, &errs)
	consumer.PushInternalMetrics(&errs)

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
	consumer := newSumConsumer(sender, nil)
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// cumulative sums get treated as regular wavefront metrics
	consumer.Consume(cumulativeMetric, &errs)
	consumer.PushInternalMetrics(&errs)

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
	consumer := newSumConsumer(sender, nil)
	assert.Equal(t, pdata.MetricDataTypeSum, consumer.Type())
	var errs []error

	// unspecified sums get treated as regular wavefront metrics
	consumer.Consume(cumulativeMetric, &errs)
	consumer.PushInternalMetrics(&errs)

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
	metric := newMetric("bad.metric", pdata.MetricDataTypeSum)
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
	consumer := newSumConsumer(sender, &consumerOptions{
		Logger:                zap.New(observedZapCore),
		ReportInternalMetrics: true,
	})
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
	consumer := newGaugeConsumer(sender, nil)

	assert.Equal(t, pdata.MetricDataTypeGauge, consumer.Type())
	var errs []error
	consumer.Consume(metric, &errs)
	consumer.PushInternalMetrics(&errs)
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

func setDataPointTimestamp(ts int64, dataPoint pdata.NumberDataPoint) {
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
