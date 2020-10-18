// Copyright 2020, OpenTelemetry Authors
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
package awsecscontainermetrics

import (
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestConvertToOTMetrics(t *testing.T) {
	timestamp := timestampProto(time.Now())
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	labelKeys := []*metricspb.LabelKey{
		{Key: "label_key_1"},
	}
	labelValues := []*metricspb.LabelValue{
		{Value: "label_value_1"},
	}

	metrics := convertToOCMetrics("container.", m, labelKeys, labelValues, timestamp)
	require.EqualValues(t, 26, len(metrics))
}

func TestIntGauge(t *testing.T) {
	intValue := uint64(100)

	labelKeys := []*metricspb.LabelKey{
		{Key: "label_key_1"},
	}
	labelValues := []*metricspb.LabelValue{
		{Value: "label_value_1"},
	}

	m := intGauge("cpu_utilized", "Count", &intValue, labelKeys, labelValues)
	require.NotNil(t, m)

	m = intGauge("cpu_utilized", "Count", nil, labelKeys, labelValues)
	require.Nil(t, m)
}

func TestDoubleGauge(t *testing.T) {
	floatValue := float64(100.01)

	labelKeys := []*metricspb.LabelKey{
		{Key: "label_key_1"},
	}
	labelValues := []*metricspb.LabelValue{
		{Value: "label_value_1"},
	}

	m := doubleGauge("cpu_utilized", "Count", &floatValue, labelKeys, labelValues)
	require.NotNil(t, m)

	m = doubleGauge("cpu_utilized", "Count", nil, labelKeys, labelValues)
	require.Nil(t, m)

}

func TestIntCumulative(t *testing.T) {
	floatValue := uint64(100)

	labelKeys := []*metricspb.LabelKey{
		{Key: "label_key_1"},
	}
	labelValues := []*metricspb.LabelValue{
		{Value: "label_value_1"},
	}

	m := intCumulative("cpu_utilized", "Count", &floatValue, labelKeys, labelValues)
	require.NotNil(t, m)

	m = intCumulative("cpu_utilized", "Count", nil, labelKeys, labelValues)
	require.Nil(t, m)

}
