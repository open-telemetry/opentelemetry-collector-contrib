// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func Test_mapToAdxMetric(t *testing.T) {
	tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tstr := ts.AsTime().Format(time.RFC3339)

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name               string
		resourceFn         func() pcommon.Resource
		metricsDataFn      func() pmetric.Metric
		expectedAdxMetrics []*AdxMetric
		configFn           func() *Config
	}{
		{
			name: "counter_over_time",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(22.0)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "counter_over_time",
					MetricType: "Sum",
					Value:      22.0,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},
		{
			name: "int_counter_over_time",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("int_counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(221)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "int_counter_over_time",
					MetricType: "Sum",
					Value:      221,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},

		{
			name: "nil_counter_over_time",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("nil_counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "simple_histogram_with_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("simple_histogram_with_value")
				histogram.SetDataType(pmetric.MetricDataTypeHistogram)
				histogramPt := histogram.Histogram().DataPoints().AppendEmpty()
				histogramPt.SetMExplicitBounds(distributionBounds)
				histogramPt.SetMBucketCounts(distributionCounts)
				histogramPt.SetSum(23)  //
				histogramPt.SetCount(7) // sum of distributionBounds
				histogramPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				return histogram
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_sum",
					MetricType: "Histogram",
					Value:      23,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_count",
					MetricType: "Histogram",
					Value:      7,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
				//The list of buckets
				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_bucket",
					MetricType: "Histogram",
					Value:      4,
					Host:       "test-host",
					Attributes: `{"key":"value","le":"1"}`,
				},

				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_bucket",
					MetricType: "Histogram",
					Value:      6,
					Host:       "test-host",
					Attributes: `{"key":"value","le":"2"}`,
				},

				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_bucket",
					MetricType: "Histogram",
					Value:      9,
					Host:       "test-host",
					Attributes: `{"key":"value","le":"4"}`,
				},

				{
					Timestamp:  tstr,
					MetricName: "simple_histogram_with_value_bucket",
					MetricType: "Histogram",
					Value:      14, // Sum of distribution counts
					Host:       "test-host",
					Attributes: `{"key":"value","le":"+Inf"}`,
				},
			},
		},
		{
			name: "nil_gauge_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("nil_gauge_value")
				gauge.SetDataType(pmetric.MetricDataTypeGauge)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "int_gauge_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("Int_gauge_value")
				gauge.SetDataType(pmetric.MetricDataTypeGauge)
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetIntVal(5)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "Int_gauge_value",
					MetricType: "Gauge",
					Value:      5,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},
		{
			name: "float_gauge_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("Float_gauge_value")
				gauge.SetDataType(pmetric.MetricDataTypeGauge)
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetDoubleVal(5.32)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "Float_gauge_value",
					MetricType: "Gauge",
					Value:      float64(5.32),
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},
		{
			name: "summary",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				summary := pmetric.NewMetric()
				summary.SetName("summary")
				summary.SetDataType(pmetric.MetricDataTypeSummary)
				summaryPt := summary.Summary().DataPoints().AppendEmpty()
				summaryPt.SetTimestamp(ts)
				summaryPt.SetStartTimestamp(ts)
				summaryPt.SetCount(2)
				summaryPt.SetSum(42)
				qt1 := summaryPt.QuantileValues().AppendEmpty()
				qt1.SetQuantile(0.5)
				qt1.SetValue(34)
				qt2 := summaryPt.QuantileValues().AppendEmpty()
				qt2.SetQuantile(0.6)
				qt2.SetValue(45)
				return summary
			},
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "summary_sum",
					MetricType: "Summary",
					Value:      float64(42),
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
				{
					Timestamp:  tstr,
					MetricName: "summary_count",
					MetricType: "Summary",
					Value:      float64(2),
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
				{
					Timestamp:  tstr,
					MetricName: "summary_0.5",
					MetricType: "Summary",
					Value:      float64(34),
					Host:       "test-host",
					Attributes: `{"key":"value","qt": "0.5","summary_0.5": 34}`,
				},
				{
					Timestamp:  tstr,
					MetricName: "summary_0.6",
					MetricType: "Summary",
					Value:      float64(45),
					Host:       "test-host",
					Attributes: `{"key":"value","qt": "0.6","summary_0.6": 45}`,
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "nil_summary",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				summary := pmetric.NewMetric()
				summary.SetName("nil_summary")
				summary.SetDataType(pmetric.MetricDataTypeSummary)
				summaryPt := summary.Summary().DataPoints().AppendEmpty()
				summaryPt.SetTimestamp(ts)
				summaryPt.SetStartTimestamp(ts)
				summaryPt.SetCount(2)
				summaryPt.SetSum(42)
				qt1 := summaryPt.QuantileValues().AppendEmpty()
				qt1.SetQuantile(0.5)
				qt1.SetValue(34)
				qt2 := summaryPt.QuantileValues().AppendEmpty()
				qt2.SetQuantile(0.6)
				qt2.SetValue(45)
				return summary
			},
			expectedAdxMetrics: nil,
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "unknown_type",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("unknown_with_dims")
				metric.SetDataType(pmetric.MetricDataTypeNone)
				return metric
			},
			expectedAdxMetrics: nil,
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.resourceFn()
			md := tt.metricsDataFn()
			actualMetrics := mapToAdxMetric(res, md, zap.NewNop())
			encoder := json.NewEncoder(ioutil.Discard)
			for i, expectedMetric := range tt.expectedAdxMetrics {
				assert.Equal(t, expectedMetric.MetricName, actualMetrics[i].MetricName)
				assert.Equal(t, expectedMetric.MetricType, actualMetrics[i].MetricType)
				assert.Equal(t, expectedMetric.Value, actualMetrics[i].Value, fmt.Sprintf("Mismatch for value for test %s", tt.name))
				assert.Equal(t, expectedMetric.Host, actualMetrics[i].Host)
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				assert.JSONEq(t, expectedMetric.Attributes, actualMetrics[i].Attributes)
				err := encoder.Encode(actualMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func newMetricsWithResources() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().InsertString("key", "value")
	res.Attributes().InsertString(hostKey, "test-host")
	return res
}
