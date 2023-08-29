// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// The timestamps used for the tests
var tsUnix = time.Unix(time.Now().Unix(), time.Now().UnixNano())
var ts = pcommon.NewTimestampFromTime(tsUnix)
var tstr = ts.AsTime().Format(time.RFC3339Nano)

// the histogram values and distribution for the tests
var distributionBounds = []float64{1, 2, 4}
var distributionCounts = []uint64{4, 2, 3, 5}

func Test_rawMetricsToAdxMetrics(t *testing.T) {
	t.Parallel()
	// Resource map
	rmap := make(map[string]interface{})
	rmap["key"] = "value"
	rmap[hostkey] = testhost

	// Metric map , with scopes
	mmap := make(map[string]interface{})
	mmap[scopename] = "SN"
	mmap[scopeversion] = "SV"

	tests := []struct {
		name               string                                                                    // name of the test
		metricsDataFn      func(metricType pmetric.MetricType, ts pcommon.Timestamp) pmetric.Metrics // function that generates the metric
		metricDataType     pmetric.MetricType
		expectedAdxMetrics []*AdxMetric // expected results
	}{
		{
			//
			name:           "metrics_counter_over_time",
			metricsDataFn:  newMetrics,
			metricDataType: pmetric.MetricTypeSum,
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "page_faults",
					MetricDescription:  "process page faults",
					MetricType:         "Sum",
					MetricValue:        22.0,
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
		{
			name:           "metrics_simple_histogram_with_value",
			metricsDataFn:  newMetrics,
			metricDataType: pmetric.MetricTypeHistogram,
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_sum",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", sumdescription),
					MetricValue:        23,
					Host:               testhost,
					MetricAttributes:   newMapFromAttr(`{"scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					ResourceAttributes: rmap,
				},
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_count",
					MetricType:         "Histogram", // There is no unit for counts. It is only a count or a "number of samples"
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricValue:        7,
					MetricUnit:         "milliseconds",
					MetricAttributes:   newMapFromAttr(`{"scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},
				// The list of buckets
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        4,
					MetricAttributes:   newMapFromAttr(`{"le":"1", "scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        6,
					MetricAttributes:   newMapFromAttr(`{"le":"2", "scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        9,
					MetricAttributes:   newMapFromAttr(`{"le":"4", "scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        14, // Sum of distribution counts
					MetricAttributes:   newMapFromAttr(`{"le":"+Inf", "scope.name":"SN", "scope.version":"SV","k1":"v1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := tt.metricsDataFn(tt.metricDataType, ts)
			actualMetrics := rawMetricsToAdxMetrics(context.Background(), metrics, zap.NewNop())
			encoder := json.NewEncoder(io.Discard)
			for i, expectedMetric := range tt.expectedAdxMetrics {
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				// Metric assertions
				assert.Equal(t, expectedMetric.MetricName, actualMetrics[i].MetricName)
				assert.Equal(t, expectedMetric.MetricType, actualMetrics[i].MetricType)
				assert.Equal(t, expectedMetric.MetricValue, actualMetrics[i].MetricValue, fmt.Sprintf("Mismatch for value for test %s", tt.name))
				assert.Equal(t, expectedMetric.MetricDescription, actualMetrics[i].MetricDescription)
				assert.Equal(t, expectedMetric.MetricUnit, actualMetrics[i].MetricUnit)
				assert.Equal(t, expectedMetric.MetricAttributes, actualMetrics[i].MetricAttributes)
				// Host as separate column
				assert.Equal(t, expectedMetric.Host, actualMetrics[i].Host)
				// Resource attributes
				assert.Equal(t, expectedMetric.ResourceAttributes, actualMetrics[i].ResourceAttributes)
				err := encoder.Encode(actualMetrics[i])
				assert.NoError(t, err)
			}
		})
	}

}

func Test_mapToAdxMetric(t *testing.T) {
	t.Parallel()

	rmap := make(map[string]interface{})
	rmap["key"] = "value"
	rmap[hostkey] = testhost
	mmap := make(map[string]interface{})

	tests := []struct {
		name               string                  // name of the test
		resourceFn         func() pcommon.Resource // function that generates the resources
		metricDataFn       func() pmetric.Metric   // function that generates the metric
		expectedAdxMetrics []*AdxMetric            // expected results
		configFn           func() *Config          // the config to apply
	}{
		{
			name:       "counter_over_time",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("page_faults")
				sumV.SetDescription("process page faults") // Only description and no units. Count units are just "number of / count of"
				sumV.SetEmptySum()
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(22.0)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "page_faults",
					MetricDescription:  "process page faults",
					MetricType:         "Sum",
					MetricValue:        22.0,
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
		{
			name:       "int_counter_over_time",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("page_faults")
				sumV.SetDescription("process page faults")
				sumV.SetEmptySum()
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(221)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "page_faults",
					MetricDescription:  "process page faults",
					MetricType:         "Sum",
					MetricValue:        221,
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},

		{
			name:       "nil_counter_over_time",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("page_faults")
				sumV.SetEmptySum()
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name:       "simple_histogram_with_value",
			resourceFn: newDummyResource,
			// Refers example from https://opentelemetry.io/docs/reference/specification/metrics/api/#instrument-unit
			metricDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("http.server.duration")
				histogram.SetUnit("milliseconds")
				histogram.SetDescription("measures the duration of the inbound HTTP request")
				histogram.SetEmptyHistogram()
				histogramPt := histogram.Histogram().DataPoints().AppendEmpty()
				histogramPt.ExplicitBounds().FromRaw(distributionBounds)
				histogramPt.BucketCounts().FromRaw(distributionCounts)
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
					Timestamp:          tstr,
					MetricName:         "http.server.duration_sum",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", sumdescription),
					MetricValue:        23,
					Host:               testhost,
					MetricAttributes:   mmap,
					ResourceAttributes: rmap,
				},
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_count",
					MetricType:         "Histogram", // There is no unit for counts. It is only a count or a "number of samples"
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricValue:        7,
					MetricUnit:         "milliseconds",
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
				// The list of buckets
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        4,
					MetricAttributes:   newMapFromAttr(`{"le":"1"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        6,
					MetricAttributes:   newMapFromAttr(`{"le":"2"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        9,
					MetricAttributes:   newMapFromAttr(`{"le":"4"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},

				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_bucket",
					MetricType:         "Histogram",
					MetricUnit:         "milliseconds",
					MetricDescription:  "measures the duration of the inbound HTTP request",
					MetricValue:        14, // Sum of distribution counts
					MetricAttributes:   newMapFromAttr(`{"le":"+Inf"}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
		{
			name:       "nil_gauge_value",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
				gauge.SetEmptyGauge()
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name:       "int_gauge_value",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
				gauge.SetEmptyGauge()
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetIntValue(5)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "cpu.frequency",
					MetricType:         "Gauge",
					MetricUnit:         "GHz",
					MetricDescription:  "the real-time CPU clock speed",
					MetricValue:        5,
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
		{
			name:       "float_gauge_value",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("cpu.frequency")
				gauge.SetUnit("GHz")
				gauge.SetDescription("the real-time CPU clock speed")
				gauge.SetEmptyGauge()
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetDoubleValue(5.32)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:          tstr,
					MetricName:         "cpu.frequency",
					MetricType:         "Gauge",
					MetricUnit:         "GHz",
					MetricDescription:  "the real-time CPU clock speed",
					MetricValue:        float64(5.32),
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
		},
		{
			name:       "summary",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				summary := pmetric.NewMetric()
				summary.SetName("http.server.duration")
				summary.SetDescription("measures the duration of the inbound HTTP request")
				summary.SetUnit("milliseconds")
				summary.SetEmptySummary()
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
					Timestamp:          tstr,
					MetricName:         "http.server.duration_sum",
					MetricType:         "Summary",
					MetricUnit:         "milliseconds",
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", sumdescription),
					MetricValue:        float64(42),
					Host:               testhost,
					MetricAttributes:   mmap,
					ResourceAttributes: rmap,
				},
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_count",
					MetricType:         "Summary",
					MetricUnit:         "milliseconds",
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricValue:        float64(2),
					MetricAttributes:   mmap,
					Host:               testhost,
					ResourceAttributes: rmap,
				},
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_0.5",
					MetricType:         "Summary",
					MetricUnit:         "milliseconds",
					MetricValue:        float64(34),
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricAttributes:   newMapFromAttr(`{"qt": "0.5","http.server.duration_0.5": 34}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},
				{
					Timestamp:          tstr,
					MetricName:         "http.server.duration_0.6",
					MetricType:         "Summary",
					MetricUnit:         "milliseconds",
					MetricValue:        float64(45),
					MetricDescription:  fmt.Sprintf("%s%s", "measures the duration of the inbound HTTP request", countdescription),
					MetricAttributes:   newMapFromAttr(`{"qt": "0.6","http.server.duration_0.6": 45}`),
					Host:               testhost,
					ResourceAttributes: rmap,
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name:       "nil_summary",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				summary := pmetric.NewMetric()
				summary.SetName("nil_summary")
				summary.SetEmptySummary()
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
			name:       "unknown_type",
			resourceFn: newDummyResource,
			metricDataFn: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("unknown_with_dims")
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
			md := tt.metricDataFn()
			emptyscopemap := make(map[string]interface{}, 2)
			actualMetrics := mapToAdxMetric(res, md, emptyscopemap, zap.NewNop())
			encoder := json.NewEncoder(io.Discard)
			for i, expectedMetric := range tt.expectedAdxMetrics {
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				// Metric assertions
				assert.Equal(t, expectedMetric.MetricName, actualMetrics[i].MetricName)
				assert.Equal(t, expectedMetric.MetricType, actualMetrics[i].MetricType)
				assert.Equal(t, expectedMetric.MetricValue, actualMetrics[i].MetricValue, fmt.Sprintf("Mismatch for value for test %s", tt.name))
				assert.Equal(t, expectedMetric.MetricDescription, actualMetrics[i].MetricDescription)
				assert.Equal(t, expectedMetric.MetricUnit, actualMetrics[i].MetricUnit)
				assert.Equal(t, expectedMetric.MetricAttributes, actualMetrics[i].MetricAttributes)
				// Host as separate column
				assert.Equal(t, expectedMetric.Host, actualMetrics[i].Host)
				// Resource attributes
				assert.Equal(t, expectedMetric.ResourceAttributes, actualMetrics[i].ResourceAttributes)
				err := encoder.Encode(actualMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func newDummyResource() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().PutStr("key", "value")
	res.Attributes().PutStr(hostkey, testhost)
	return res
}

func newMapFromAttr(jsonStr string) map[string]interface{} {
	dynamic := make(map[string]interface{})
	err := json.Unmarshal([]byte(jsonStr), &dynamic)
	// If there is a failure , send the error back in a map
	if err != nil {
		return map[string]interface{}{"err": err.Error()}
	}
	return dynamic
}

func newMetrics(metricType pmetric.MetricType, ts pcommon.Timestamp) pmetric.Metrics {
	// Create metrics
	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	rms.Resource().Attributes().PutStr("key", "value")
	rms.Resource().Attributes().PutStr(hostkey, testhost)
	// Scope metric in a metric
	sms := rms.ScopeMetrics().AppendEmpty()
	scope := sms.Scope()
	scope.SetName("SN")
	scope.SetVersion("SV")
	//

	switch metricType {
	case pmetric.MetricTypeSum:
		sumV := sms.Metrics().AppendEmpty()
		sumV.SetName("page_faults")
		sumV.SetDescription("process page faults") // Only description and no units. Count units are just "number of / count of"
		sumV.SetEmptySum()
		dp := sumV.Sum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(22.0)
		dp.SetTimestamp(ts)
	case pmetric.MetricTypeHistogram:
		histogram := sms.Metrics().AppendEmpty()
		histogram.SetName("http.server.duration")
		histogram.SetUnit("milliseconds")
		histogram.SetDescription("measures the duration of the inbound HTTP request")
		histogram.SetEmptyHistogram()
		histogramPt := histogram.Histogram().DataPoints().AppendEmpty()
		histogramPt.ExplicitBounds().FromRaw(distributionBounds)
		histogramPt.BucketCounts().FromRaw(distributionCounts)
		histogramPt.Attributes().PutStr("k1", "v1")
		histogramPt.SetSum(23)  //
		histogramPt.SetCount(7) // sum of distributionBounds
		histogramPt.SetTimestamp(pcommon.NewTimestampFromTime(ts.AsTime()))
	}
	return metrics
}
