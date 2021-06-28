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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_metricDataToSplunk(t *testing.T) {
	logger := zap.NewNop()

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	ts := pdata.TimestampFromTime(tsUnix)
	tsMSecs := timestampToSecondsWithMillisecondPrecision(ts)

	doubleVal := 1234.5678
	int64Val := int64(123)

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name                     string
		metricsDataFn            func() pdata.Metrics
		wantSplunkMetrics        []*splunk.Event
		wantNumDroppedTimeseries int
	}{
		{
			name: "empty_resource_metrics",
			metricsDataFn: func() pdata.Metrics {
				metrics := pdata.NewMetrics()
				metrics.ResourceMetrics().AppendEmpty()
				return metrics
			},
		},
		{
			name: "nil_instrumentation_library_metrics",
			metricsDataFn: func() pdata.Metrics {
				return newMetricsWithResources()
			},
		},
		{
			name: "nil_metric",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().AppendEmpty()
				return metrics
			},
			wantNumDroppedTimeseries: 1,
		},
		{
			name: "nil_double_gauge_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleGauge := ilm.Metrics().AppendEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				return metrics
			},
		},
		{
			name: "nil_int_gauge_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intGauge := ilm.Metrics().AppendEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				return metrics
			},
		},
		{
			name: "nil_int_histogram_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intHistogram := ilm.Metrics().AppendEmpty()
				intHistogram.SetName("hist_int_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				return metrics
			},
		},
		{
			name: "nil_double_histogram_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleHistogram := ilm.Metrics().AppendEmpty()
				doubleHistogram.SetName("hist_double_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeHistogram)
				return metrics
			},
		},
		{
			name: "nil_double_sum_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleSum := ilm.Metrics().AppendEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				return metrics
			},
		},
		{
			name: "nil_int_sum_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intSum := ilm.Metrics().AppendEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				return metrics
			},
		},
		{
			name: "double_gauge_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleGauge := ilm.Metrics().AppendEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleGauge.DoubleGauge().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "int_gauge_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intGauge := ilm.Metrics().AppendEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intGauge.IntGauge().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "int_histogram_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intHistogram := ilm.Metrics().AppendEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "double_histogram_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleHistogram := ilm.Metrics().AppendEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeHistogram)
				doubleHistogram.Histogram().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "int_sum_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intSum := ilm.Metrics().AppendEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intSum.IntSum().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "double_sum_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleSum := ilm.Metrics().AppendEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleSum.DoubleSum().DataPoints().AppendEmpty()
				return metrics
			},
		},
		{
			name: "gauges",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().InsertString("service.name", "mysource")
				rm.Resource().Attributes().InsertString("host.name", "myhost")
				rm.Resource().Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
				rm.Resource().Attributes().InsertString("com.splunk.index", "myindex")
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()

				doubleGauge := ilm.Metrics().AppendEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleDataPt := doubleGauge.DoubleGauge().DataPoints().AppendEmpty()
				doubleDataPt.SetValue(doubleVal)
				doubleDataPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))

				intGauge := ilm.Metrics().AppendEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intDataPt := intGauge.IntGauge().DataPoints().AppendEmpty()
				intDataPt.SetValue(int64Val)
				intDataPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
				intDataPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))

				return metrics
			},
			wantSplunkMetrics: []*splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"com.splunk.index", "com.splunk.sourcetype", "host.name", "service.name", "k0", "k1", "metric_type"}, []interface{}{"myindex", "mysourcetype", "myhost", "mysource", "v0", "v1", "DoubleGauge"}, doubleVal, "mysource", "mysourcetype", "myindex", "myhost"),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, []string{"com.splunk.index", "com.splunk.sourcetype", "host.name", "service.name", "k0", "k1", "metric_type"}, []interface{}{"myindex", "mysourcetype", "myhost", "mysource", "v0", "v1", "IntGauge"}, int64Val, "mysource", "mysourcetype", "myindex", "myhost"),
			},
		},

		{
			name: "double_histogram_no_upper_bound",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleHistogram := ilm.Metrics().AppendEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeHistogram)
				doubleHistogramPt := doubleHistogram.Histogram().DataPoints().AppendEmpty()
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
				return metrics
			},
		},
		{
			name: "double_histogram",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleHistogram := ilm.Metrics().AppendEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeHistogram)
				doubleHistogramPt := doubleHistogram.Histogram().DataPoints().AppendEmpty()
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts(distributionCounts)
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
				return metrics
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
		},
		{
			name: "int_histogram_no_upper_bound",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intHistogram := ilm.Metrics().AppendEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogramPt := intHistogram.IntHistogram().DataPoints().AppendEmpty()
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
				return metrics
			},
		},

		{
			name: "int_histogram",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intHistogram := ilm.Metrics().AppendEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogramPt := intHistogram.IntHistogram().DataPoints().AppendEmpty()
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts(distributionCounts)
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
				return metrics
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
						"metric_name:int_histogram_with_dims_sum": int64(23),
						"metric_type": "IntHistogram",
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
						"metric_name:int_histogram_with_dims_count": uint64(7),
						"metric_type": "IntHistogram",
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
						"metric_name:int_histogram_with_dims_bucket": uint64(4),
						"metric_type": "IntHistogram",
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
						"metric_name:int_histogram_with_dims_bucket": uint64(6),
						"metric_type": "IntHistogram",
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
						"metric_name:int_histogram_with_dims_bucket": uint64(9),
						"metric_type": "IntHistogram",
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
						"metric_name:int_histogram_with_dims_bucket": uint64(14),
						"metric_type": "IntHistogram",
					},
				},
			},
		},
		{
			name: "int_sum",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				intSum := ilm.Metrics().AppendEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intDataPt := intSum.IntSum().DataPoints().AppendEmpty()
				intDataPt.SetTimestamp(ts)
				intDataPt.SetValue(62)
				return metrics
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
						"metric_type":                   "IntSum",
					},
				},
			},
		},
		{
			name: "double_sum",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleSum := ilm.Metrics().AppendEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleDataPt := doubleSum.DoubleSum().DataPoints().AppendEmpty()
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetValue(62)
				return metrics
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
						"metric_type":                      "DoubleSum",
					},
				},
			},
		},
		{
			name: "summary",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				summary := ilm.Metrics().AppendEmpty()
				summary.SetName("summary")
				summary.SetDataType(pdata.MetricDataTypeSummary)
				doubleDataPt := summary.Summary().DataPoints().AppendEmpty()
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetStartTimestamp(ts)
				doubleDataPt.SetCount(2)
				doubleDataPt.SetSum(42)
				qt1 := doubleDataPt.QuantileValues().AppendEmpty()
				qt1.SetQuantile(0.5)
				qt1.SetValue(34)
				qt2 := doubleDataPt.QuantileValues().AppendEmpty()
				qt2.SetQuantile(0.6)
				qt2.SetValue(45)
				return metrics
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
		},
		{
			name: "unknown_type",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				doubleSum := ilm.Metrics().AppendEmpty()
				doubleSum.SetName("unknown_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeNone)
				return metrics
			},
			wantNumDroppedTimeseries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := tt.metricsDataFn()
			gotMetrics, gotNumDroppedTimeSeries := metricDataToSplunk(logger, md, &Config{})
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeSeries)
			for i, want := range tt.wantSplunkMetrics {
				assert.Equal(t, want, gotMetrics[i])
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

func newMetricsWithResources() pdata.Metrics {
	metrics := pdata.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	rm.Resource().Attributes().InsertString("k1", "v1")
	rm.InstrumentationLibraryMetrics().AppendEmpty()
	return metrics
}
