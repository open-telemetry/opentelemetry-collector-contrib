// Copyright  The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
)

func TestNewMetricDataType(t *testing.T) {
	metricDataType := NewMetricDataType(pdata.MetricDataTypeGauge, pdata.MetricAggregationTemporalityDelta, true)

	require.NotNil(t, metricDataType)
	assert.Equal(t, metricDataType.MetricDataType(), pdata.MetricDataTypeGauge)
	assert.Equal(t, metricDataType.AggregationTemporality(), pdata.MetricAggregationTemporalityDelta)
	assert.True(t, metricDataType.IsMonotonic())
}

func TestMetricValueDataType_MetricDataType(t *testing.T) {
	valueDataType := metricValueDataType{dataType: pdata.MetricDataTypeGauge}

	assert.Equal(t, valueDataType.MetricDataType(), pdata.MetricDataTypeGauge)
}

func TestMetricValueDataType_AggregationTemporality(t *testing.T) {
	valueDataType := metricValueDataType{aggregationTemporality: pdata.MetricAggregationTemporalityDelta}

	assert.Equal(t, valueDataType.AggregationTemporality(), pdata.MetricAggregationTemporalityDelta)
}

func TestMetricValueDataType_IsMonotonic(t *testing.T) {
	valueDataType := metricValueDataType{isMonotonic: true}

	assert.True(t, valueDataType.IsMonotonic())
}
