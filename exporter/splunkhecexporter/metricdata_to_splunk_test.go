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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_metricDataToSplunk(t *testing.T) {
	logger := zap.NewNop()

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	ts := pdata.TimestampUnixNano(tsUnix.UnixNano())
	tsMSecs := timestampToSecondsWithMillisecondPrecision(ts)

	doubleVal := 1234.5678
	int64Val := int64(123)

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name                     string
		metricsDataFn            func() pdata.Metrics
		wantSplunkMetrics        []splunk.Event
		wantNumDroppedTimeseries int
	}{
		{
			name: "empty_resource_metrics",
			metricsDataFn: func() pdata.Metrics {
				metrics := pdata.NewMetrics()
				metrics.ResourceMetrics().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_instrumentation_library_metrics",
			metricsDataFn: func() pdata.Metrics {
				return newMetricsWithResources()
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_metric",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				return metrics
			},
			wantSplunkMetrics:        []splunk.Event{},
			wantNumDroppedTimeseries: 1,
		},
		{
			name: "nil_double_gauge_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleGauge := ilm.Metrics().At(0)
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_gauge_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intGauge := ilm.Metrics().At(0)
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_histogram_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intHistogram := ilm.Metrics().At(0)
				intHistogram.SetName("hist_int_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_double_histogram_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleHistogram := ilm.Metrics().At(0)
				doubleHistogram.SetName("hist_double_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_double_sum_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleSum := ilm.Metrics().At(0)
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_sum_value",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intSum := ilm.Metrics().At(0)
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_gauge_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleGauge := ilm.Metrics().At(0)
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleGauge.DoubleGauge().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_gauge_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intGauge := ilm.Metrics().At(0)
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intGauge.IntGauge().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_histogram_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intHistogram := ilm.Metrics().At(0)
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_histogram_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleHistogram := ilm.Metrics().At(0)
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_sum_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intSum := ilm.Metrics().At(0)
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intSum.IntSum().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_sum_empty_data_point",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleSum := ilm.Metrics().At(0)
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleSum.DoubleSum().DataPoints().Resize(1)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "gauges",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				metrics.ResourceMetrics().Resize(1)
				rm := metrics.ResourceMetrics().At(0)
				rm.Resource().Attributes().InsertString("service.name", "mysource")
				rm.Resource().Attributes().InsertString("host.name", "myhost")
				rm.Resource().Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
				rm.Resource().Attributes().InsertString("com.splunk.index", "myindex")
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				rm.InstrumentationLibraryMetrics().Resize(1)
				ilm := rm.InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(2)

				doubleGauge := ilm.Metrics().At(0)
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleGauge.DoubleGauge().DataPoints().Resize(1)
				doubleDataPt := doubleGauge.DoubleGauge().DataPoints().At(0)
				doubleDataPt.SetValue(doubleVal)
				doubleDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))

				intGauge := ilm.Metrics().At(1)
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intGauge.IntGauge().DataPoints().Resize(1)
				intDataPt := intGauge.IntGauge().DataPoints().At(0)
				intDataPt.SetValue(int64Val)
				intDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				intDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))

				return metrics
			},
			wantSplunkMetrics: []splunk.Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"com.splunk.index", "com.splunk.sourcetype", "host.name", "service.name", "k0", "k1"}, []interface{}{"myindex", "mysourcetype", "myhost", "mysource", "v0", "v1"}, doubleVal, "mysource", "mysourcetype", "myindex", "myhost"),
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, []string{"com.splunk.index", "com.splunk.sourcetype", "host.name", "service.name", "k0", "k1"}, []interface{}{"myindex", "mysourcetype", "myhost", "mysource", "v0", "v1"}, int64Val, "mysource", "mysourcetype", "myindex", "myhost"),
			},
		},

		{
			name: "double_histogram_no_upper_bound",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleHistogram := ilm.Metrics().At(0)
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().DataPoints().Resize(1)
				doubleHistogramPt := doubleHistogram.DoubleHistogram().DataPoints().At(0)
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_histogram",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleHistogram := ilm.Metrics().At(0)
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().DataPoints().Resize(1)
				doubleHistogramPt := doubleHistogram.DoubleHistogram().DataPoints().At(0)
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts(distributionCounts)
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{
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
					},
				},
			},
		},
		{
			name: "int_histogram_no_upper_bound",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intHistogram := ilm.Metrics().At(0)
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().DataPoints().Resize(1)
				intHistogramPt := intHistogram.IntHistogram().DataPoints().At(0)
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},

		{
			name: "int_histogram",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intHistogram := ilm.Metrics().At(0)
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().DataPoints().Resize(1)
				intHistogramPt := intHistogram.IntHistogram().DataPoints().At(0)
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts(distributionCounts)
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{
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
					},
				},
			},
		},
		{
			name: "int_sum",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				intSum := ilm.Metrics().At(0)
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intSum.IntSum().DataPoints().Resize(1)
				intDataPt := intSum.IntSum().DataPoints().At(0)
				intDataPt.SetTimestamp(ts)
				intDataPt.SetValue(62)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{
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
					},
				},
			},
		},
		{
			name: "double_sum",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleSum := ilm.Metrics().At(0)
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleSum.DoubleSum().DataPoints().Resize(1)
				doubleDataPt := doubleSum.DoubleSum().DataPoints().At(0)
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetValue(62)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{
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
					},
				},
			},
		},
		{
			name: "unknown_type",
			metricsDataFn: func() pdata.Metrics {
				metrics := newMetricsWithResources()
				ilm := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				doubleSum := ilm.Metrics().At(0)
				doubleSum.SetName("unknown_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeNone)
				return metrics
			},
			wantSplunkMetrics:        []splunk.Event{},
			wantNumDroppedTimeseries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := tt.metricsDataFn()
			gotMetrics, gotNumDroppedTimeSeries, err := metricDataToSplunk(logger, md, &Config{})
			assert.NoError(t, err)
			assert.Equal(t, tt.wantNumDroppedTimeseries, gotNumDroppedTimeSeries)
			for i, want := range tt.wantSplunkMetrics {
				assert.Equal(t, &want, gotMetrics[i])
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
) splunk.Event {
	fields := map[string]interface{}{fmt.Sprintf("metric_name:%s", metricName): val}

	for i, k := range keys {
		fields[k] = values[i]
	}

	return splunk.Event{
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
	ts := pdata.TimestampUnixNano(32001000345)
	assert.Equal(t, 32.001, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRounding(t *testing.T) {
	ts := pdata.TimestampUnixNano(32001999345)
	assert.Equal(t, 32.002, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRoundingWithNanos(t *testing.T) {
	ts := pdata.TimestampUnixNano(9999999999991500001)
	assert.Equal(t, 9999999999.992, *timestampToSecondsWithMillisecondPrecision(ts))
}

func TestNilTimeWhenTimestampIsZero(t *testing.T) {
	ts := pdata.TimestampUnixNano(0)
	assert.Nil(t, timestampToSecondsWithMillisecondPrecision(ts))
}

func newMetricsWithResources() pdata.Metrics {
	metrics := pdata.NewMetrics()
	metrics.ResourceMetrics().Resize(1)
	rm := metrics.ResourceMetrics().At(0)
	rm.Resource().Attributes().InsertString("k0", "v0")
	rm.Resource().Attributes().InsertString("k1", "v1")
	rm.InstrumentationLibraryMetrics().Resize(1)
	return metrics
}
