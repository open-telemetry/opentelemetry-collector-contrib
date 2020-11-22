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
			name: "nil_resource_metrics",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				metrics.ResourceMetrics().Append(rm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_instrumentation_library_metrics",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_metric",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleGauge := pdata.NewMetric()
				ilm.Metrics().Append(doubleGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_double_gauge_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleGauge := pdata.NewMetric()
				doubleGauge.InitEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				ilm.Metrics().Append(doubleGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_gauge_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intGauge := pdata.NewMetric()
				intGauge.InitEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				ilm.Metrics().Append(intGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_histogram_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intHistogram := pdata.NewMetric()
				intHistogram.InitEmpty()
				intHistogram.SetName("hist_int_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				ilm.Metrics().Append(intHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_double_histogram_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleHistogram := pdata.NewMetric()
				doubleHistogram.InitEmpty()
				doubleHistogram.SetName("hist_double_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				ilm.Metrics().Append(doubleHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_double_sum_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleSum := pdata.NewMetric()
				doubleSum.InitEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				ilm.Metrics().Append(doubleSum)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "nil_int_sum_value",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intSum := pdata.NewMetric()
				intSum.InitEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				ilm.Metrics().Append(intSum)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_gauge_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleGauge := pdata.NewMetric()
				doubleGauge.InitEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleGauge.DoubleGauge().InitEmpty()
				doubleDataPt := pdata.NewDoubleDataPoint()
				doubleGauge.DoubleGauge().DataPoints().Append(doubleDataPt)
				ilm.Metrics().Append(doubleGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_gauge_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intGauge := pdata.NewMetric()
				intGauge.InitEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intGauge.IntGauge().InitEmpty()
				intDataPt := pdata.NewIntDataPoint()
				intGauge.IntGauge().DataPoints().Append(intDataPt)
				ilm.Metrics().Append(intGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_histogram_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intHistogram := pdata.NewMetric()
				intHistogram.InitEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().InitEmpty()
				histogramDataPt := pdata.NewIntHistogramDataPoint()
				intHistogram.IntHistogram().DataPoints().Append(histogramDataPt)
				ilm.Metrics().Append(intHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_histogram_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleHistogram := pdata.NewMetric()
				doubleHistogram.InitEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().InitEmpty()
				doubleHistogramDataPt := pdata.NewDoubleHistogramDataPoint()
				doubleHistogram.DoubleHistogram().DataPoints().Append(doubleHistogramDataPt)
				ilm.Metrics().Append(doubleHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "int_sum_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intSum := pdata.NewMetric()
				intSum.InitEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intSum.IntSum().InitEmpty()
				sumDataPt := pdata.NewIntDataPoint()
				intSum.IntSum().DataPoints().Append(sumDataPt)
				ilm.Metrics().Append(intSum)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_sum_nil_data_point",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleSum := pdata.NewMetric()
				doubleSum.InitEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleSum.DoubleSum().InitEmpty()
				doubleDataPt := pdata.NewDoubleDataPoint()
				doubleSum.DoubleSum().DataPoints().Append(doubleDataPt)
				ilm.Metrics().Append(doubleSum)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "gauges",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				rm.Resource().Attributes().InsertString("service.name", "mysource")
				rm.Resource().Attributes().InsertString("host.name", "myhost")
				rm.Resource().Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
				rm.Resource().Attributes().InsertString("com.splunk.index", "myindex")

				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleGauge := pdata.NewMetric()
				doubleGauge.InitEmpty()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleGauge.SetDataType(pdata.MetricDataTypeDoubleGauge)
				doubleGauge.DoubleGauge().InitEmpty()
				doubleDataPt := pdata.NewDoubleDataPoint()
				doubleDataPt.InitEmpty()
				doubleDataPt.SetValue(doubleVal)
				doubleDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				doubleGauge.DoubleGauge().DataPoints().Append(doubleDataPt)
				ilm.Metrics().Append(doubleGauge)

				intGauge := pdata.NewMetric()
				intGauge.InitEmpty()
				intGauge.SetName("gauge_int_with_dims")
				intGauge.SetDataType(pdata.MetricDataTypeIntGauge)
				intGauge.IntGauge().InitEmpty()
				intDataPt := pdata.NewIntDataPoint()
				intDataPt.InitEmpty()
				intDataPt.SetValue(int64Val)
				intDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				intDataPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				intGauge.IntGauge().DataPoints().Append(intDataPt)
				ilm.Metrics().Append(intGauge)

				rm.InstrumentationLibraryMetrics().Append(ilm)
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

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleHistogram := pdata.NewMetric()
				doubleHistogram.InitEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().InitEmpty()
				doubleHistogramPt := pdata.NewDoubleHistogramDataPoint()
				doubleHistogramPt.InitEmpty()
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				doubleHistogram.DoubleHistogram().DataPoints().Append(doubleHistogramPt)
				ilm.Metrics().Append(doubleHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},
		{
			name: "double_histogram",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleHistogram := pdata.NewMetric()
				doubleHistogram.InitEmpty()
				doubleHistogram.SetName("double_histogram_with_dims")
				doubleHistogram.SetDataType(pdata.MetricDataTypeDoubleHistogram)
				doubleHistogram.DoubleHistogram().InitEmpty()
				doubleHistogramPt := pdata.NewDoubleHistogramDataPoint()
				doubleHistogramPt.InitEmpty()
				doubleHistogramPt.SetExplicitBounds(distributionBounds)
				doubleHistogramPt.SetBucketCounts(distributionCounts)
				doubleHistogramPt.SetSum(23)
				doubleHistogramPt.SetCount(7)
				doubleHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				doubleHistogram.DoubleHistogram().DataPoints().Append(doubleHistogramPt)
				ilm.Metrics().Append(doubleHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
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

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intHistogram := pdata.NewMetric()
				intHistogram.InitEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().InitEmpty()
				intHistogramPt := pdata.NewIntHistogramDataPoint()
				intHistogramPt.InitEmpty()
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts([]uint64{4, 2, 3})
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				intHistogram.IntHistogram().DataPoints().Append(intHistogramPt)
				ilm.Metrics().Append(intHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
				return metrics
			},
			wantSplunkMetrics: []splunk.Event{},
		},

		{
			name: "int_histogram",
			metricsDataFn: func() pdata.Metrics {

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				intHistogram := pdata.NewMetric()
				intHistogram.InitEmpty()
				intHistogram.SetName("int_histogram_with_dims")
				intHistogram.SetDataType(pdata.MetricDataTypeIntHistogram)
				intHistogram.IntHistogram().InitEmpty()
				intHistogramPt := pdata.NewIntHistogramDataPoint()
				intHistogramPt.InitEmpty()
				intHistogramPt.SetExplicitBounds(distributionBounds)
				intHistogramPt.SetBucketCounts(distributionCounts)
				intHistogramPt.SetCount(7)
				intHistogramPt.SetSum(23)
				intHistogramPt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
				intHistogram.IntHistogram().DataPoints().Append(intHistogramPt)
				ilm.Metrics().Append(intHistogram)

				rm.InstrumentationLibraryMetrics().Append(ilm)
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

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()
				rm.InstrumentationLibraryMetrics().Append(ilm)

				intSum := pdata.NewMetric()
				intSum.InitEmpty()
				intSum.SetName("int_sum_with_dims")
				intSum.SetDataType(pdata.MetricDataTypeIntSum)
				intSum.IntSum().InitEmpty()
				intDataPt := pdata.NewIntDataPoint()
				intDataPt.InitEmpty()
				intDataPt.SetTimestamp(ts)
				intDataPt.SetValue(62)
				intSum.IntSum().DataPoints().Append(intDataPt)
				ilm.Metrics().Append(intSum)

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

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleSum := pdata.NewMetric()
				doubleSum.InitEmpty()
				doubleSum.SetName("double_sum_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeDoubleSum)
				doubleSum.DoubleSum().InitEmpty()
				doubleDataPt := pdata.NewDoubleDataPoint()
				doubleDataPt.InitEmpty()
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetValue(62)
				doubleSum.DoubleSum().DataPoints().Append(doubleDataPt)
				ilm.Metrics().Append(doubleSum)
				rm.InstrumentationLibraryMetrics().Append(ilm)
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

				metrics := pdata.NewMetrics()
				rm := pdata.NewResourceMetrics()
				rm.InitEmpty()
				metrics.ResourceMetrics().Append(rm)
				rm.Resource().Attributes().InsertString("k0", "v0")
				rm.Resource().Attributes().InsertString("k1", "v1")
				ilm := pdata.NewInstrumentationLibraryMetrics()
				ilm.InitEmpty()

				doubleSum := pdata.NewMetric()
				doubleSum.InitEmpty()
				doubleSum.SetName("unknown_with_dims")
				doubleSum.SetDataType(pdata.MetricDataTypeNone)
				ilm.Metrics().Append(doubleSum)
				rm.InstrumentationLibraryMetrics().Append(ilm)
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
