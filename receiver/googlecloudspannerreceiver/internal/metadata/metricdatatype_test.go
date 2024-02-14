// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewMetricType(t *testing.T) {
	metricDataType := NewMetricType(pmetric.MetricTypeGauge, pmetric.AggregationTemporalityDelta, true)

	require.NotNil(t, metricDataType)
	assert.Equal(t, metricDataType.MetricType(), pmetric.MetricTypeGauge)
	assert.Equal(t, metricDataType.AggregationTemporality(), pmetric.AggregationTemporalityDelta)
	assert.True(t, metricDataType.IsMonotonic())
}

func TestMetricValueDataType_MetricType(t *testing.T) {
	valueDataType := metricValueDataType{dataType: pmetric.MetricTypeGauge}

	assert.Equal(t, valueDataType.MetricType(), pmetric.MetricTypeGauge)
}

func TestMetricValueDataType_AggregationTemporality(t *testing.T) {
	valueDataType := metricValueDataType{aggregationTemporality: pmetric.AggregationTemporalityDelta}

	assert.Equal(t, valueDataType.AggregationTemporality(), pmetric.AggregationTemporalityDelta)
}

func TestMetricValueDataType_IsMonotonic(t *testing.T) {
	valueDataType := metricValueDataType{isMonotonic: true}

	assert.True(t, valueDataType.IsMonotonic())
}
