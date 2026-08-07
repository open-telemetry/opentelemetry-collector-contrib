// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	testhost = "test-host"
)

// The timestamps used for the tests
var (
	tsUnix = time.Unix(time.Now().Unix(), time.Now().UnixNano())
	ts     = pcommon.NewTimestampFromTime(tsUnix)
	tstr   = ts.AsTime().Format(time.RFC3339Nano)
)

// the histogram values and distribution for the tests
var (
	distributionBounds = []float64{1, 2, 4}
	distributionCounts = []uint64{4, 2, 3, 5}
)

func Test_rawMetricsToAdxMetrics(t *testing.T) {
	t.Parallel()
	// Resource map
	rmap := make(map[string]any)
	rmap["key"] = "value"
	rmap[hostkey] = testhost

	// Metric map , with scopes
	mmap := make(map[string]any)
	mmap[scopename] = "SN"
	mmap[scopeversion] = "SV"

	tests := []struct {
		name               string                                                                    // name of the test
		metricsDataFn      func(metricType pmetric.MetricType, ts pcommon.Timestamp) pmetric.Metrics // function that generates the metric
		metricDataType     pmetric.MetricType
		expectedAdxMetrics []*adxMetric // expected results
	}{
		{
			//
			name:           "metrics_counter_over_time",
			metricsDataFn:  newMetrics,
			metricDataType: pmetric.MetricTypeSum,
			expectedAdxMetrics: []*adxMetric{
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
			expectedAdxMetrics: []*adxMetric{
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
			actualMetrics := rawMetricsToAdxMetrics(t.Context(), metrics, false, zap.NewNop())
			encoder := json.NewEncoder(io.Discard)
			for i, expectedMetric := range tt.expectedAdxMetrics {
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				// Metric assertions
				assert.Equal(t, expectedMetric.MetricName, actualMetrics[i].MetricName)
				assert.Equal(t, expectedMetric.MetricType, actualMetrics[i].MetricType)
				assert.Equalf(t, expectedMetric.MetricValue, actualMetrics[i].MetricValue, "Mismatch for value for test %s", tt.name)
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

	rmap := make(map[string]any)
	rmap["key"] = "value"
	rmap[hostkey] = testhost
	mmap := make(map[string]any)

	tests := []struct {
		name               string                  // name of the test
		resourceFn         func() pcommon.Resource // function that generates the resources
		metricDataFn       func() pmetric.Metric   // function that generates the metric
		expectedAdxMetrics []*adxMetric            // expected results
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

			expectedAdxMetrics: []*adxMetric{
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

			expectedAdxMetrics: []*adxMetric{
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

			expectedAdxMetrics: []*adxMetric{
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
			expectedAdxMetrics: []*adxMetric{
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
			expectedAdxMetrics: []*adxMetric{
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
			expectedAdxMetrics: []*adxMetric{
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
			emptyscopemap := make(map[string]any, 2)
			actualMetrics := mapToAdxMetric(res, md, emptyscopemap, false, zap.NewNop())
			encoder := json.NewEncoder(io.Discard)
			for i, expectedMetric := range tt.expectedAdxMetrics {
				assert.Equal(t, expectedMetric.Timestamp, actualMetrics[i].Timestamp)
				// Metric assertions
				assert.Equal(t, expectedMetric.MetricName, actualMetrics[i].MetricName)
				assert.Equal(t, expectedMetric.MetricType, actualMetrics[i].MetricType)
				assert.Equalf(t, expectedMetric.MetricValue, actualMetrics[i].MetricValue, "Mismatch for value for test %s", tt.name)
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

func newMapFromAttr(jsonStr string) map[string]any {
	dynamic := make(map[string]any)
	err := json.Unmarshal([]byte(jsonStr), &dynamic)
	// If there is a failure , send the error back in a map
	if err != nil {
		return map[string]any{"err": err.Error()}
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

func Test_extractExemplars(t *testing.T) {
	t.Parallel()

	// no exemplars -> nil
	emptyDp := pmetric.NewNumberDataPoint()
	assert.Nil(t, extractExemplars(emptyDp.Exemplars()))

	dp := pmetric.NewNumberDataPoint()
	ex := dp.Exemplars().AppendEmpty()
	ex.SetDoubleValue(2820)
	ex.SetTimestamp(ts)
	ex.SetTraceID(pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
	ex.SetSpanID(pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))
	ex.FilteredAttributes().PutStr("http.route", "/api/checkout")

	got := extractExemplars(dp.Exemplars())
	assert.Len(t, got, 1)
	assert.InDelta(t, 2820.0, got[0]["value"], 0.001)
	assert.Equal(t, tstr, got[0]["timestamp"])
	assert.Equal(t, "00000000000000000000000000000001", got[0]["trace_id"])
	assert.Equal(t, "0000000000000002", got[0]["span_id"])
	assert.Equal(t, map[string]any{"http.route": "/api/checkout"}, got[0]["attributes"])

	// int-valued exemplar
	intDp := pmetric.NewNumberDataPoint()
	iex := intDp.Exemplars().AppendEmpty()
	iex.SetIntValue(42)
	iex.SetTimestamp(ts)
	gotInt := extractExemplars(intDp.Exemplars())
	assert.Len(t, gotInt, 1)
	assert.Equal(t, int64(42), gotInt[0]["value"])
}

// Integer exemplars must not be widened to float64, which would silently lose
// precision above 2^53.
func Test_extractExemplars_int64Precision(t *testing.T) {
	t.Parallel()

	const beyondFloat64Mantissa = int64(9007199254740993) // 2^53 + 1
	dp := pmetric.NewNumberDataPoint()
	ex := dp.Exemplars().AppendEmpty()
	ex.SetIntValue(beyondFloat64Mantissa)
	ex.SetTimestamp(ts)

	got := extractExemplars(dp.Exemplars())
	assert.Len(t, got, 1)
	assert.Equal(t, beyondFloat64Mantissa, got[0]["value"])

	encoded, err := jsoniter.MarshalToString(got[0])
	require.NoError(t, err)
	assert.Contains(t, encoded, "9007199254740993")
}

// A non-finite exemplar value or attribute must not fail serialization of the whole
// metric row, since that would silently drop an otherwise valid metric.
func Test_extractExemplars_nonFiniteValuesAreSerializable(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		value    float64
		expected string
	}{
		{"nan", math.NaN(), "NaN"},
		{"positive infinity", math.Inf(1), "Infinity"},
		{"negative infinity", math.Inf(-1), "-Infinity"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dp := pmetric.NewNumberDataPoint()
			ex := dp.Exemplars().AppendEmpty()
			ex.SetDoubleValue(tt.value)
			ex.SetTimestamp(ts)
			ex.FilteredAttributes().PutDouble("bad.attribute", tt.value)

			got := extractExemplars(dp.Exemplars())
			assert.Len(t, got, 1)
			assert.Equal(t, tt.expected, got[0]["value"])
			assert.Equal(t, tt.expected, got[0]["attributes"].(map[string]any)["bad.attribute"])

			// The whole row must still serialize; previously this returned an error
			// and the metric was dropped.
			adxMetrics := mapToAdxMetric(newDummyResource(), newSumWithExemplar(tt.value), map[string]any{}, true, zap.NewNop())
			require.Len(t, adxMetrics, 1)
			encoded, err := jsoniter.MarshalToString(adxMetrics[0])
			require.NoError(t, err)
			assert.Contains(t, encoded, tt.expected)
		})
	}
}

// An exemplar with no recorded value omits the value key rather than reporting a synthetic 0.
func Test_extractExemplars_emptyValueType(t *testing.T) {
	t.Parallel()

	dp := pmetric.NewNumberDataPoint()
	ex := dp.Exemplars().AppendEmpty()
	ex.SetTimestamp(ts)
	require.Equal(t, pmetric.ExemplarValueTypeEmpty, ex.ValueType())

	got := extractExemplars(dp.Exemplars())
	assert.Len(t, got, 1)
	assert.NotContains(t, got[0], "value")
}

func newSumWithExemplar(value float64) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName("http.server.duration")
	m.SetEmptySum()
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp.SetTimestamp(ts)
	ex := dp.Exemplars().AppendEmpty()
	ex.SetDoubleValue(value)
	ex.SetTimestamp(ts)
	return m
}

func Test_mapToAdxMetric_includeExemplars(t *testing.T) {
	t.Parallel()

	res := newDummyResource()
	emptyscopemap := map[string]any{"scope.name": "", "scope.version": ""}

	// A Sum data point carrying one exemplar.
	newSum := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("http.server.duration")
		m.SetEmptySum()
		dp := m.Sum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(2820)
		dp.SetTimestamp(ts)
		ex := dp.Exemplars().AppendEmpty()
		ex.SetDoubleValue(2820)
		ex.SetTimestamp(ts)
		ex.SetTraceID(pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
		ex.SetSpanID(pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))
		return m
	}

	// Disabled (default): exemplars are dropped, preserving existing behavior.
	off := mapToAdxMetric(res, newSum(), emptyscopemap, false, zap.NewNop())
	assert.Len(t, off, 1)
	assert.Nil(t, off[0].Exemplars)

	// Enabled: exemplars are carried with the metric.
	on := mapToAdxMetric(res, newSum(), emptyscopemap, true, zap.NewNop())
	assert.Len(t, on, 1)
	assert.Len(t, on[0].Exemplars, 1)
	assert.Equal(t, "00000000000000000000000000000001", on[0].Exemplars[0]["trace_id"])
	assert.Equal(t, "0000000000000002", on[0].Exemplars[0]["span_id"])

	// Histogram: exemplars attach to the representative _count row only.
	newHist := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("http.server.duration")
		m.SetEmptyHistogram()
		dp := m.Histogram().DataPoints().AppendEmpty()
		dp.ExplicitBounds().FromRaw(distributionBounds)
		dp.BucketCounts().FromRaw(distributionCounts)
		dp.SetSum(23)
		dp.SetCount(14)
		dp.SetTimestamp(ts)
		ex := dp.Exemplars().AppendEmpty()
		ex.SetDoubleValue(2820)
		ex.SetTimestamp(ts)
		ex.SetTraceID(pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
		return m
	}
	hist := mapToAdxMetric(res, newHist(), emptyscopemap, true, zap.NewNop())
	var countRows, exemplarRows int
	for _, m := range hist {
		if m.MetricName == "http.server.duration_count" {
			countRows++
			assert.Len(t, m.Exemplars, 1, "exemplars should be on the _count row")
		} else {
			assert.Nil(t, m.Exemplars, "exemplars should not be on %s", m.MetricName)
		}
		exemplarRows += len(m.Exemplars)
	}
	assert.Equal(t, 1, countRows)
	assert.Equal(t, 1, exemplarRows, "exemplars must appear exactly once per histogram point")
}
