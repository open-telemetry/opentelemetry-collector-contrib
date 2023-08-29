// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestInt64MetricValueMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, IntValueType)

	assert.Equal(t, metricName, metadata.Name())
	assert.Equal(t, metricColumnName, metadata.ColumnName())
	assert.Equal(t, metricDataType, metadata.DataType())
	assert.Equal(t, metricUnit, metadata.Unit())
	assert.Equal(t, IntValueType, metadata.ValueType())

	var expectedType *int64

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestFloat64MetricValueMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, FloatValueType)

	assert.Equal(t, metricName, metadata.Name())
	assert.Equal(t, metricColumnName, metadata.ColumnName())
	assert.Equal(t, metricDataType, metadata.DataType())
	assert.Equal(t, metricUnit, metadata.Unit())
	assert.Equal(t, FloatValueType, metadata.ValueType())

	var expectedType *float64

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestNullFloat64MetricValueMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, NullFloatValueType)

	assert.Equal(t, metricName, metadata.Name())
	assert.Equal(t, metricColumnName, metadata.ColumnName())
	assert.Equal(t, metricDataType, metadata.DataType())
	assert.Equal(t, metricUnit, metadata.Unit())
	assert.Equal(t, NullFloatValueType, metadata.ValueType())

	var expectedType *spanner.NullFloat64

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestUnknownMetricValueMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, err := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, UnknownValueType)

	require.Error(t, err)
	require.Nil(t, metadata)
}

func TestInt64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, IntValueType)
	metricValue := int64MetricValue{
		metadata: metadata,
		value:    int64Value,
	}

	assert.Equal(t, int64Value, metricValue.Value())
	assert.Equal(t, IntValueType, metadata.ValueType())

	dataPoint := pmetric.NewNumberDataPoint()

	metricValue.SetValueTo(dataPoint)

	assert.Equal(t, int64Value, dataPoint.IntValue())
}

func TestFloat64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, FloatValueType)
	metricValue := float64MetricValue{
		metadata: metadata,
		value:    float64Value,
	}

	assert.Equal(t, float64Value, metricValue.Value())
	assert.Equal(t, FloatValueType, metadata.ValueType())

	dataPoint := pmetric.NewNumberDataPoint()

	metricValue.SetValueTo(dataPoint)

	assert.Equal(t, float64Value, dataPoint.DoubleValue())
}

func TestNullFloat64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, NullFloatValueType)

	validNullFloat := spanner.NullFloat64{Float64: float64Value, Valid: true}
	metricValue := nullFloat64MetricValue{
		metadata: metadata,
		value:    validNullFloat,
	}
	assert.Equal(t, validNullFloat, metricValue.Value())
	assert.Equal(t, NullFloatValueType, metadata.ValueType())
	dataPoint := pmetric.NewNumberDataPoint()
	metricValue.SetValueTo(dataPoint)
	assert.Equal(t, float64Value, dataPoint.DoubleValue())

	invalidNullFloat := spanner.NullFloat64{Float64: float64Value, Valid: false}
	metricValue = nullFloat64MetricValue{
		metadata: metadata,
		value:    invalidNullFloat,
	}
	assert.Equal(t, invalidNullFloat, metricValue.Value())
	assert.Equal(t, NullFloatValueType, metadata.ValueType())
	metricValue.SetValueTo(dataPoint)
	assert.Equal(t, defaultNullFloat64Value, dataPoint.DoubleValue())
}

func TestNewInt64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, IntValueType)
	value := int64Value
	valueHolder := &value

	metricValue := newInt64MetricValue(metadata, valueHolder)

	assert.Equal(t, int64Value, metricValue.Value())
	assert.Equal(t, IntValueType, metadata.ValueType())
}

func TestNewFloat64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, FloatValueType)
	value := float64Value
	valueHolder := &value

	metricValue := newFloat64MetricValue(metadata, valueHolder)

	assert.Equal(t, float64Value, metricValue.Value())
	assert.Equal(t, FloatValueType, metadata.ValueType())
}

func TestNewNullFloat64MetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, NullFloatValueType)
	value := spanner.NullFloat64{Float64: float64Value, Valid: true}
	valueHolder := &value

	metricValue := newNullFloat64MetricValue(metadata, valueHolder)

	assert.Equal(t, value, metricValue.Value())
	assert.Equal(t, NullFloatValueType, metadata.ValueType())
}
