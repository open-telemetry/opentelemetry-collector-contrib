// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
