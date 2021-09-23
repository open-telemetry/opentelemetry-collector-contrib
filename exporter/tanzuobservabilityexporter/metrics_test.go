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
	"go.opentelemetry.io/collector/model/pdata"
)

func TestMetricsConsumerNormal(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	sum1 := newMetric("sum1", pdata.MetricDataTypeSum)
	gauge2 := newMetric("gauge2", pdata.MetricDataTypeGauge)
	sum2 := newMetric("sum2", pdata.MetricDataTypeSum)
	mockGaugeConsumer := &mockMetricConsumer{typ: pdata.MetricDataTypeGauge}
	mockSumConsumer := &mockMetricConsumer{typ: pdata.MetricDataTypeSum}
	sender := &mockFlushCloser{}
	metrics := constructMetrics(gauge1, sum1, gauge2, sum2)
	consumer := newMetricsConsumer(
		[]metricConsumer{mockGaugeConsumer, mockSumConsumer}, sender)
	assert.NoError(t, consumer.Consume(context.Background(), metrics))
	assert.ElementsMatch(
		t, []string{"gauge1", "gauge2"}, mockGaugeConsumer.names)
	assert.ElementsMatch(
		t, []string{"sum1", "sum2"}, mockSumConsumer.names)
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

func TestMetricsConsumerErrorOnFlush(t *testing.T) {
	sender := &mockFlushCloser{errorOnFlush: true}
	metrics := constructMetrics()
	consumer := newMetricsConsumer(nil, sender)
	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Equal(t, 1, sender.numFlushCalls)
}

func TestMetricsConsumerBadMetricType(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer(nil, nil)
	assert.Error(t, consumer.Consume(context.Background(), metrics))
}

func TestMetricsConsumerErrorConsuming(t *testing.T) {
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockMetricConsumer{
		typ: pdata.MetricDataTypeGauge, errorOnConsume: true}
	metrics := constructMetrics(gauge1)
	consumer := newMetricsConsumer([]metricConsumer{mockGaugeConsumer}, nil)
	assert.Error(t, consumer.Consume(context.Background(), metrics))
	assert.Len(t, mockGaugeConsumer.names, 1)
}

func TestMetricsConsumerRespectContext(t *testing.T) {
	sender := &mockFlushCloser{}
	gauge1 := newMetric("gauge1", pdata.MetricDataTypeGauge)
	mockGaugeConsumer := &mockMetricConsumer{typ: pdata.MetricDataTypeGauge}
	consumer := newMetricsConsumer([]metricConsumer{mockGaugeConsumer}, sender)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, consumer.Consume(ctx, constructMetrics(gauge1)))
	assert.Zero(t, sender.numFlushCalls)
	assert.Empty(t, mockGaugeConsumer.names)
}

func TestGaugeConsumerNormal(t *testing.T) {
	verifyGaugeConsumer(t, false)
}

func TestGaugeConsumerErrorSending(t *testing.T) {
	verifyGaugeConsumer(t, true)
}

func verifyGaugeConsumer(t *testing.T, errorOnSend bool) {
	metric := newMetric("test.metric", pdata.MetricDataTypeGauge)
	dataPoints := metric.Gauge().DataPoints()
	dataPoints.EnsureCapacity(2)
	addDataPoint(
		7,
		1631205001,
		map[string]interface{}{
			"env":    "prod",
			"bucket": 73,
		},
		dataPoints,
	)
	addDataPoint(
		7.5,
		1631205002,
		map[string]interface{}{
			"env":    "prod",
			"bucket": 73,
		},
		dataPoints,
	)
	expected := []singleMetric{
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
	consumer := newGaugeConsumer(sender)
	assert.Equal(t, pdata.MetricDataTypeGauge, consumer.Type())
	var errs []error
	consumer.Consume(metric, &errs)
	assert.ElementsMatch(t, expected, sender.metrics)
	if errorOnSend {
		assert.Len(t, errs, 2)
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
	slice pdata.NumberDataPointSlice) {
	dataPoint := slice.AppendEmpty()
	setDataPointValue(value, dataPoint)
	setDataPointTimestamp(ts, dataPoint)
	setTags(tags, dataPoint)
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

func setTags(tags map[string]interface{}, dataPoint pdata.NumberDataPoint) {
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
	attributeMap.CopyTo(dataPoint.Attributes())
}

type singleMetric struct {
	Name   string
	Value  float64
	Ts     int64
	Source string
	Tags   map[string]string
}

type mockGaugeSender struct {
	errorOnSend bool
	metrics     []singleMetric
}

func (m *mockGaugeSender) SendMetric(
	name string, value float64, ts int64, source string, tags map[string]string) error {
	tagsCopy := make(map[string]string, len(tags))
	for k, v := range tags {
		tagsCopy[k] = v
	}
	m.metrics = append(m.metrics, singleMetric{
		Name:   name,
		Value:  value,
		Ts:     ts,
		Source: source,
		Tags:   tagsCopy,
	})
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

type mockMetricConsumer struct {
	typ            pdata.MetricDataType
	errorOnConsume bool
	names          []string
}

func (m *mockMetricConsumer) Type() pdata.MetricDataType {
	return m.typ
}

func (m *mockMetricConsumer) Consume(metric pdata.Metric, errs *[]error) {
	m.names = append(m.names, metric.Name())
	if m.errorOnConsume {
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
