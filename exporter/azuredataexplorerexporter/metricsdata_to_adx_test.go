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

const (
	// The value of "type" key in configuration.
	testhost = "test-host"
)

func Test_mapToAdxMetric(t *testing.T) {
	tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tstr := ts.AsTime().Format(time.RFC3339)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name               string                  // name of the test
		resourceFn         func() pcommon.Resource // function that generates the resources
		metricsDataFn      func() pmetric.Metric   // function that generates the metric
		expectedAdxMetrics []*AdxMetric            // expected results
		configFn           func() *Config          // the config to apply
	}{
		{
			name: "counter_over_time",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("page_faults")
				sumV.SetDescription("process page faults") // Only description and no units. Count units are just "number of / count of"
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
					Timestamp:         tstr,
					MetricName:        "page_faults",
					MetricDescription: "process page faults",
					MetricType:        "Sum",
					MetricValue:       22.0,
					Host:              testhost,
					MetricAttributes:  tmap,
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
				sumV.SetName("page_faults")
				sumV.SetDescription("process page faults")
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
					Timestamp:         tstr,
					MetricName:        "page_faults",
					MetricDescription: "process page faults",
					MetricType:        "Sum",
					MetricValue:       221,
					Host:              testhost,
					MetricAttributes:  tmap,
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
				sumV.SetName("page_faults")
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
			// Refers example from https://opentelemetry.io/docs/reference/specification/metrics/api/#instrument-unit
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("http.server.duration")
				histogram.SetUnit("milliseconds")
				histogram.SetDescription("measures the duration of the inbound HTTP request")
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
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_sum",
					MetricType:        "Histogram",
					MetricUnit:        "milliseconds",
					MetricDescription: fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", sumdescription),
					MetricValue:       23,
					Host:              testhost,
					MetricAttributes:  tmap,
				},
				{
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_count",
					MetricType:        "Histogram", // There is no unit for counts. It is only a count or a "number of samples"
					MetricDescription: fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricValue:       7,
					Host:              testhost,
					MetricAttributes:  tmap,
				},
				//The list of buckets
				{
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_bucket",
					MetricType:        "Histogram",
					MetricUnit:        "milliseconds",
					MetricDescription: "measures the duration of the inbound HTTP request",
					MetricValue:       4,
					Host:              testhost,
					MetricAttributes:  newMapFromAttr(`{"key":"value","le":"1"}`),
				},

				{
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_bucket",
					MetricType:        "Histogram",
					MetricUnit:        "milliseconds",
					MetricDescription: "measures the duration of the inbound HTTP request",
					MetricValue:       6,
					Host:              testhost,
					MetricAttributes:  newMapFromAttr(`{"key":"value","le":"2"}`),
				},

				{
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_bucket",
					MetricType:        "Histogram",
					MetricUnit:        "milliseconds",
					MetricDescription: "measures the duration of the inbound HTTP request",
					MetricValue:       9,
					Host:              testhost,
					MetricAttributes:  newMapFromAttr(`{"key":"value","le":"4"}`),
				},

				{
					Timestamp:         tstr,
					MetricName:        "simple_histogram_with_value_bucket",
					MetricType:        "Histogram",
					MetricUnit:        "milliseconds",
					MetricDescription: "measures the duration of the inbound HTTP request",
					MetricValue:       14, // Sum of distribution counts
					Host:              testhost,
					MetricAttributes:  newMapFromAttr(`{"key":"value","le":"+Inf"}`),
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
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
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
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
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
					Timestamp:         tstr,
					MetricName:        "cpu.frequency",
					MetricType:        "Gauge",
					MetricUnit:        "GHz",
					MetricDescription: "the real-time CPU clock speed",
					MetricValue:       5,
					Host:              testhost,
					MetricAttributes:  tmap,
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
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
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
					Timestamp:         tstr,
					MetricName:        "cpu.frequency",
					MetricType:        "Gauge",
					MetricUnit:        "GHz",
					MetricDescription: "the real-time CPU clock speed",
					MetricValue:       float64(5.32),
					Host:              testhost,
					MetricAttributes:  tmap,
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
				summary.SetName("http.server.duration")
				summary.SetDescription("measures the duration of the inbound HTTP request")
				summary.SetUnit("milliseconds")
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
					Timestamp:         tstr,
					MetricName:        "http.server.duration_sum",
					MetricType:        "Summary",
					MetricUnit:        "milliseconds",
					MetricDescription: fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", sumdescription),
					MetricValue:       float64(42),
					Host:              testhost,
					MetricAttributes:  tmap,
				},
				{
					Timestamp:         tstr,
					MetricName:        "http.server.duration_count",
					MetricType:        "Summary",
					MetricDescription: fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricValue:       float64(2),
					Host:              testhost,
					MetricAttributes:  tmap,
				},
				{
					Timestamp:        tstr,
					MetricName:       "summary_0.5",
					MetricType:       "Summary",
					MetricValue:      float64(34),
					Host:             testhost,
					MetricAttributes: newMapFromAttr(`{"key":"value","qt": "0.5","summary_0.5": 34}`),
				},
				{
					Timestamp:        tstr,
					MetricName:       "summary_0.6",
					MetricType:       "Summary",
					MetricValue:      float64(45),
					Host:             testhost,
					MetricAttributes: newMapFromAttr(`{"key":"value","qt": "0.6","summary_0.6": 45}`),
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
				assert.Equal(t, expectedMetric.MetricValue, actualMetrics[i].MetricValue, fmt.Sprintf("Mismatch for value for test %s", tt.name))
				assert.Equal(t, expectedMetric.Host, actualMetrics[i].Host)
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				assert.Equal(t, expectedMetric.MetricAttributes, actualMetrics[i].MetricAttributes)
				assert.Equal(t, expectedMetric.MetricDescription, actualMetrics[i].MetricDescription)
				assert.Equal(t, expectedMetric.MetricUnit, actualMetrics[i].MetricUnit)
				err := encoder.Encode(actualMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func newMetricsWithResources() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().InsertString("key", "value")
	res.Attributes().InsertString(hostkey, testhost)
	return res
}

func newMapFromAttr(jsonStr string) map[string]interface{} {
	dynamic := make(map[string]interface{})
	json.Unmarshal([]byte(jsonStr), &dynamic)
	return dynamic
}
