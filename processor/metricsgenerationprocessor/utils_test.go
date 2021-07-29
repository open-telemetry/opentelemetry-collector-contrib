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

package metricsgenerationprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestCalculateValue(t *testing.T) {
	value := calculateValue(100.0, 5.0, "add", zap.NewNop(), "test_metric")
	require.Equal(t, 105.0, value)

	value = calculateValue(100.0, 5.0, "subtract", zap.NewNop(), "test_metric")
	require.Equal(t, 95.0, value)

	value = calculateValue(100.0, 5.0, "multiply", zap.NewNop(), "test_metric")
	require.Equal(t, 500.0, value)

	value = calculateValue(100.0, 5.0, "divide", zap.NewNop(), "test_metric")
	require.Equal(t, 20.0, value)

	value = calculateValue(10.0, 200.0, "percent", zap.NewNop(), "test_metric")
	require.Equal(t, 5.0, value)

	value = calculateValue(100.0, 0, "divide", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)

	value = calculateValue(100.0, 0, "percent", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)

	value = calculateValue(100.0, 0, "invalid", zap.NewNop(), "test_metric")
	require.Equal(t, 0.0, value)
}

func TestGetMetricValueWithNoDataPoint(t *testing.T) {
	md := pdata.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
	m := ms.AppendEmpty()
	m.SetName("metric_1")
	m.SetDataType(pdata.MetricDataTypeGauge)

	value := getMetricValue(md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0))
	require.Equal(t, 0.0, value)
}
