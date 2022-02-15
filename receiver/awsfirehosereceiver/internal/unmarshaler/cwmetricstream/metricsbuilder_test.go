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

package cwmetricstream

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

const (
	testRegion     = "us-east-1"
	testAccountID  = "1234567890"
	testStreamName = "MyMetricStream"
)

func TestToSemConvAttributeKey(t *testing.T) {
	testCases := map[string]struct {
		key  string
		want string
	}{
		"WithValidKey": {
			key:  "InstanceId",
			want: conventions.AttributeServiceInstanceID,
		},
		"WithInvalidKey": {
			key:  "CustomDimension",
			want: "CustomDimension",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := ToSemConvAttributeKey(testCase.key)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestMetricBuilder(t *testing.T) {
	t.Run("WithSingleMetric", func(t *testing.T) {
		metric := cWMetric{
			MetricName: "name",
			Unit:       "unit",
			Timestamp:  time.Now().UnixMilli(),
			Value:      testCWMetricValue(),
			Dimensions: map[string]string{"CustomDimension": "test"},
		}
		mb := newMetricBuilder(metric.MetricName, metric.Unit)
		mb.AddDataPoint(metric)
		got := pdata.NewMetric()
		mb.Build(got)
		require.Equal(t, metric.MetricName, got.Name())
		require.Equal(t, metric.Unit, got.Unit())
		require.Equal(t, pdata.MetricDataTypeSummary, got.DataType())
		dps := got.Summary().DataPoints()
		require.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		require.Equal(t, uint64(metric.Value.Count), dp.Count())
		require.Equal(t, metric.Value.Sum, dp.Sum())
		qv := dp.QuantileValues()
		require.Equal(t, 2, qv.Len())
		require.Equal(t, []float64{metric.Value.Min, metric.Value.Max}, []float64{qv.At(0).Value(), qv.At(1).Value()})
		require.Equal(t, 1, dp.Attributes().Len())
	})
	t.Run("WithTimestampCollision", func(t *testing.T) {
		timestamp := time.Now().UnixMilli()
		metrics := []cWMetric{
			{
				Timestamp:  timestamp,
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			},
			{
				Timestamp:  timestamp,
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			},
		}
		mb := newMetricBuilder("name", "unit")
		for _, metric := range metrics {
			mb.AddDataPoint(metric)
		}
		got := pdata.NewMetric()
		mb.Build(got)
		dps := got.Summary().DataPoints()
		require.Equal(t, 1, dps.Len())
		dp := dps.At(0)
		require.Equal(t, uint64(metrics[0].Value.Count), dp.Count())
		require.Equal(t, metrics[0].Value.Sum, dp.Sum())
		require.Equal(t, 0, dp.Attributes().Len())
	})
}

func TestResourceMetricsBuilder(t *testing.T) {
	testCases := map[string]struct {
		namespace      string
		wantAttributes map[string]string
	}{
		"WithAwsNamespace": {
			namespace: "AWS/EC2",
			wantAttributes: map[string]string{
				attributeAWSCloudWatchMetricStreamName: testStreamName,
				conventions.AttributeCloudAccountID:    testAccountID,
				conventions.AttributeCloudRegion:       testRegion,
				conventions.AttributeServiceName:       "EC2",
				conventions.AttributeServiceNamespace:  "AWS",
			},
		},
		"WithCustomNamespace": {
			namespace: "CustomNamespace",
			wantAttributes: map[string]string{
				attributeAWSCloudWatchMetricStreamName: testStreamName,
				conventions.AttributeCloudAccountID:    testAccountID,
				conventions.AttributeCloudRegion:       testRegion,
				conventions.AttributeServiceName:       "CustomNamespace",
				conventions.AttributeServiceNamespace:  "",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metric := cWMetric{
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			}
			rmb := newResourceMetricsBuilder(testStreamName, testAccountID, testRegion, testCase.namespace)
			rmb.AddMetric(metric)
			got := pdata.NewResourceMetrics()
			rmb.Build(got)
			attrs := got.Resource().Attributes()
			for k, want := range testCase.wantAttributes {
				v, ok := attrs.Get(k)
				if want != "" {
					require.True(t, ok)
					require.Equal(t, want, v.AsString())
				} else {
					require.False(t, ok)
				}
			}
		})
	}
	t.Run("WithSameMetricDifferentDimensions", func(t *testing.T) {
		metrics := []cWMetric{
			{
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{},
			},
			{
				MetricName: "name",
				Unit:       "unit",
				Timestamp:  time.Now().Add(time.Second * 3).UnixMilli(),
				Value:      testCWMetricValue(),
				Dimensions: map[string]string{
					"CustomDimension": "value",
				},
			},
		}
		rmb := newResourceMetricsBuilder(testStreamName, testAccountID, testRegion, "AWS/EC2")
		for _, metric := range metrics {
			rmb.AddMetric(metric)
		}
		got := pdata.NewResourceMetrics()
		rmb.Build(got)
		require.Equal(t, 1, got.InstrumentationLibraryMetrics().Len())
		gotMetrics := got.InstrumentationLibraryMetrics().At(0).Metrics()
		require.Equal(t, 1, gotMetrics.Len())
		dps := gotMetrics.At(0).Summary().DataPoints()
		require.Equal(t, 2, dps.Len())
	})
}

// testCWMetricValue is a convenience function for creating a test cWMetricValue
func testCWMetricValue() *cWMetricValue {
	return &cWMetricValue{100, 0, float64(rand.Int63n(100)), float64(rand.Int63n(4))}
}
