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

package splunkhecexporter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_metricDataToSplunk(t *testing.T) {
	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	ts := pdata.NewTimestampFromTime(tsUnix)
	tsMSecs := timestampToSecondsWithMillisecondPrecision(ts)

	doubleVal := 1234.5678
	int64Val := int64(123)

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name              string
		resourceFn        func() pdata.Resource
		metricsDataFn     func() pdata.Metric
		wantSplunkMetrics []*splunk.Event
		configFn          func() *Config
	}{
		{
			name: "nil_gauge_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				gauge := pdata.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetDataType(pdata.MetricDataTypeGauge)
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "nan_gauge_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				gauge := pdata.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetDataType(pdata.MetricDataTypeGauge)
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				dp.SetDoubleVal(math.NaN())
				return gauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, "NaN", "", "", "", "unknown"),
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "+Inf_gauge_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				gauge := pdata.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetDataType(pdata.MetricDataTypeGauge)
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				dp.SetDoubleVal(math.Inf(1))
				return gauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, "+Inf", "", "", "", "unknown"),
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "-Inf_gauge_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				gauge := pdata.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetDataType(pdata.MetricDataTypeGauge)
				dp := gauge.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				dp.SetDoubleVal(math.Inf(-1))
				return gauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, "-Inf", "", "", "", "unknown"),
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "nil_histogram_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				histogram := pdata.NewMetric()
				histogram.SetName("histogram_with_dims")
				histogram.SetDataType(pdata.MetricDataTypeHistogram)
				return histogram
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "nil_sum_value",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				sum := pdata.NewMetric()
				sum.SetName("sum_with_dims")
				sum.SetDataType(pdata.MetricDataTypeSum)
				return sum
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "gauge_empty_data_point",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				gauge := pdata.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetDataType(pdata.MetricDataTypeGauge)
				gauge.Gauge().DataPoints().AppendEmpty()
				return gauge
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "histogram_empty_data_point",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				histogram := pdata.NewMetric()
				histogram.SetName("histogram_with_dims")
				histogram.SetDataType(pdata.MetricDataTypeHistogram)
				histogram.Histogram().DataPoints().AppendEmpty()
				return histogram
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "sum_empty_data_point",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				sum := pdata.NewMetric()
				sum.SetName("sum_with_dims")
				sum.SetDataType(pdata.MetricDataTypeSum)
				sum.Sum().DataPoints().AppendEmpty()
				return sum
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "int_gauge",
			resourceFn: func() pdata.Resource {
				res := pdata.NewResource()
				res.Attributes().InsertString("com.splunk.source", "mysource")
				res.Attributes().InsertString("host.name", "myhost")
				res.Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
				res.Attributes().InsertString("com.splunk.index", "myindex")
				res.Attributes().InsertString("k0", "v0")
				res.Attributes().InsertString("k1", "v1")
				return res
			},
			metricsDataFn: func() pdata.Metric {
				intGauge := pdata.NewMetric()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeGauge)
				intDataPt := intGauge.Gauge().DataPoints().AppendEmpty()
				intDataPt.SetIntVal(int64Val)
				intDataPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				intDataPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))

				return intGauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, int64Val, "mysource", "mysourcetype", "myindex", "myhost"),
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},

		{
			name: "double_gauge",
			resourceFn: func() pdata.Resource {
				res := pdata.NewResource()
				res.Attributes().InsertString("com.splunk.source", "mysource")
				res.Attributes().InsertString("host.name", "myhost")
				res.Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
				res.Attributes().InsertString("com.splunk.index", "myindex")
				res.Attributes().InsertString("k0", "v0")
				res.Attributes().InsertString("k1", "v1")
				return res
			},
			metricsDataFn: func() pdata.Metric {

				doubleGauge := pdata.NewMetric()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeGauge)
				doubleDataPt := doubleGauge.Gauge().DataPoints().AppendEmpty()
				doubleDataPt.SetDoubleVal(doubleVal)
				doubleDataPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))

				return doubleGauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, doubleVal, "mysource", "mysourcetype", "myindex", "myhost"),
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},

		{
			name: "histogram_no_upper_bound",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				histogram := pdata.NewMetric()
				histogram.SetName("double_histogram_with_dims")
				histogram.SetDataType(pdata.MetricDataTypeHistogram)
				histogramPt := histogram.Histogram().DataPoints().AppendEmpty()
				histogramPt.SetExplicitBounds(distributionBounds)
				histogramPt.SetBucketCounts([]uint64{4, 2, 3})
				histogramPt.SetSum(23)
				histogramPt.SetCount(7)
				histogramPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				return histogram
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "histogram",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				histogram := pdata.NewMetric()
				histogram.SetName("double_histogram_with_dims")
				histogram.SetDataType(pdata.MetricDataTypeHistogram)
				histogramPt := histogram.Histogram().DataPoints().AppendEmpty()
				histogramPt.SetExplicitBounds(distributionBounds)
				histogramPt.SetBucketCounts(distributionCounts)
				histogramPt.SetSum(23)
				histogramPt.SetCount(7)
				histogramPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))
				return histogram
			},
			wantSplunkMetrics: []*splunk.Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"metric_name:double_histogram_with_dims_sum": float64(23),
						"metric_type": "Histogram",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"metric_name:double_histogram_with_dims_count": uint64(7),
						"metric_type": "Histogram",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"le": "1",
						"metric_name:double_histogram_with_dims_bucket": uint64(4),
						"metric_type": "Histogram",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"le": "2",
						"metric_name:double_histogram_with_dims_bucket": uint64(6),
						"metric_type": "Histogram",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"le": "4",
						"metric_name:double_histogram_with_dims_bucket": uint64(9),
						"metric_type": "Histogram",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0": "v0",
						"k1": "v1",
						"le": "+Inf",
						"metric_name:double_histogram_with_dims_bucket": uint64(14),
						"metric_type": "Histogram",
					},
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},

		{
			name: "int_sum",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				intSum := pdata.NewMetric()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeSum)
				intDataPt := intSum.Sum().DataPoints().AppendEmpty()
				intDataPt.SetTimestamp(ts)
				intDataPt.SetIntVal(62)
				return intSum
			},
			wantSplunkMetrics: []*splunk.Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                            "v0",
						"k1":                            "v1",
						"metric_name:int_sum_with_dims": int64(62),
						"metric_type":                   "Sum",
					},
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "double_sum",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				doubleSum := pdata.NewMetric()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeSum)
				doubleDataPt := doubleSum.Sum().DataPoints().AppendEmpty()
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetDoubleVal(62)
				return doubleSum
			},
			wantSplunkMetrics: []*splunk.Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                               "v0",
						"k1":                               "v1",
						"metric_name:double_sum_with_dims": float64(62),
						"metric_type":                      "Sum",
					},
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "summary",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				summary := pdata.NewMetric()
				summary.SetName("summary")
				summary.SetDataType(pdata.MetricDataTypeSummary)
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
			wantSplunkMetrics: []*splunk.Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                      "v0",
						"k1":                      "v1",
						"metric_name:summary_sum": float64(42),
						"metric_type":             "Summary",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                        "v0",
						"k1":                        "v1",
						"metric_name:summary_count": uint64(2),
						"metric_type":               "Summary",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                      "v0",
						"k1":                      "v1",
						"qt":                      "0.5",
						"metric_name:summary_0.5": float64(34),
						"metric_type":             "Summary",
					},
				},
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]interface{}{
						"k0":                      "v0",
						"k1":                      "v1",
						"qt":                      "0.6",
						"metric_name:summary_0.6": float64(45),
						"metric_type":             "Summary",
					},
				},
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
		{
			name: "unknown_type",
			resourceFn: func() pdata.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetName("unknown_with_dims")
				metric.SetDataType(pdata.MetricDataTypeNone)
				return metric
			},
			wantSplunkMetrics: nil,
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},

		{
			name: "custom_config_mapping",
			resourceFn: func() pdata.Resource {
				res := pdata.NewResource()
				res.Attributes().InsertString("mysource", "mysource2")
				res.Attributes().InsertString("myhost", "myhost2")
				res.Attributes().InsertString("mysourcetype", "mysourcetype2")
				res.Attributes().InsertString("myindex", "myindex2")
				res.Attributes().InsertString("k0", "v0")
				res.Attributes().InsertString("k1", "v1")
				return res
			},
			metricsDataFn: func() pdata.Metric {
				doubleGauge := pdata.NewMetric()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeGauge)
				doubleDataPt := doubleGauge.Gauge().DataPoints().AppendEmpty()
				doubleDataPt.SetDoubleVal(doubleVal)
				doubleDataPt.SetTimestamp(pdata.NewTimestampFromTime(tsUnix))

				return doubleGauge
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []interface{}{"v0", "v1", "Gauge"}, doubleVal, "mysource2", "mysourcetype2", "myindex2", "myhost2"),
			},
			configFn: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.HecToOtelAttrs.SourceType = "mysourcetype"
				cfg.HecToOtelAttrs.Source = "mysource"
				cfg.HecToOtelAttrs.Host = "myhost"
				cfg.HecToOtelAttrs.Index = "myindex"
				return cfg
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.resourceFn()
			md := tt.metricsDataFn()
			cfg := tt.configFn()
			gotMetrics := mapMetricToSplunkEvent(res, md, cfg, zap.NewNop())
			encoder := json.NewEncoder(ioutil.Discard)
			for i, want := range tt.wantSplunkMetrics {
				assert.Equal(t, want, gotMetrics[i])
				err := encoder.Encode(gotMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func commonSplunkMetric(
	metricName string,
	ts *float64,
	keys []string,
	values []interface{},
	val interface{},
	source string,
	sourcetype string,
	index string,
	host string,
) *splunk.Event {
	fields := map[string]interface{}{fmt.Sprintf("metric_name:%s", metricName): val}

	for i, k := range keys {
		fields[k] = values[i]
	}

	return &splunk.Event{
		Time:       ts,
		Source:     source,
		SourceType: sourcetype,
		Index:      index,
		Host:       host,
		Event:      "metric",
		Fields:     fields,
	}
}

func TestTimestampFormat(t *testing.T) {
	ts := pdata.Timestamp(32001000345)
	assert.Equal(t, 32.001, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRounding(t *testing.T) {
	ts := pdata.Timestamp(32001999345)
	assert.Equal(t, 32.002, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRoundingWithNanos(t *testing.T) {
	ts := pdata.Timestamp(9999999999991500001)
	assert.Equal(t, 9999999999.992, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestNilTimeWhenTimestampIsZero(t *testing.T) {
	ts := pdata.Timestamp(0)
	assert.Nil(t, timestampToSecondsWithMillisecondPrecision(ts))
}

func newMetricsWithResources() pdata.Resource {
	res := pdata.NewResource()
	res.Attributes().InsertString("k0", "v0")
	res.Attributes().InsertString("k1", "v1")
	return res
}
