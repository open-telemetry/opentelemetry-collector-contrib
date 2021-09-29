// Copyright 2019, OpenTelemetry Authors
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

package splunkhecreceiver

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_splunkV2ToMetricsData(t *testing.T) {
	// Timestamps for Splunk have a resolution to the millisecond, where the time is reported in seconds with a floating value to the millisecond.
	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	nanos := int64(sec * 1e9)

	buildDefaultSplunkDataPt := func() *splunk.Event {
		return &splunk.Event{
			Time:       &sec,
			Host:       "localhost",
			Source:     "source",
			SourceType: "sourcetype",
			Index:      "index",
			Event:      "metrics",
			Fields: map[string]interface{}{
				"metric_name:single": int64Ptr(13),
				"k0":                 "v0",
				"k1":                 "v1",
				"k2":                 "v2",
			},
		}
	}

	tests := []struct {
		name                  string
		splunkDataPoint       *splunk.Event
		wantMetricsData       pdata.Metrics
		wantDroppedTimeseries int
		hecConfig             *Config
	}{
		{
			name:            "int_gauge",
			splunkDataPoint: buildDefaultSplunkDataPt(),
			wantMetricsData: buildDefaultMetricsData(nanos),
			hecConfig:       defaultTestingHecConfig,
		},
		{
			name: "multiple",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:yetanother"] = int64Ptr(14)
				pt.Fields["metric_name:yetanotherandanother"] = int64Ptr(15)
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				metrics := buildDefaultMetricsData(nanos)
				mts := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

				metricPt := mts.AppendEmpty()
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("yetanother")
				intPt := metricPt.Gauge().DataPoints().AppendEmpty()
				intPt.SetIntVal(14)
				intPt.SetTimestamp(pdata.Timestamp(nanos))
				intPt.Attributes().InsertString("k0", "v0")
				intPt.Attributes().InsertString("k1", "v1")
				intPt.Attributes().InsertString("k2", "v2")

				metricPt2 := mts.AppendEmpty()
				metricPt2.SetDataType(pdata.MetricDataTypeGauge)
				metricPt2.SetName("yetanotherandanother")
				intPt2 := metricPt2.Gauge().DataPoints().AppendEmpty()
				intPt2.SetIntVal(15)
				intPt2.SetTimestamp(pdata.Timestamp(nanos))
				intPt2.Attributes().InsertString("k0", "v0")
				intPt2.Attributes().InsertString("k1", "v1")
				intPt2.Attributes().InsertString("k2", "v2")

				return metrics
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "double_gauge",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = float64Ptr(13.13)
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("single")
				doublePt := metricPt.Gauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleVal(13.13)
				doublePt.SetTimestamp(pdata.Timestamp(nanos))
				doublePt.Attributes().InsertString("k0", "v0")
				doublePt.Attributes().InsertString("k1", "v1")
				doublePt.Attributes().InsertString("k2", "v2")
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "int_counter_pointer",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				return pt
			}(),
			wantMetricsData: buildDefaultMetricsData(nanos),
			hecConfig:       defaultTestingHecConfig,
		},
		{
			name: "int_counter",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = int64(13)
				return pt
			}(),
			wantMetricsData: buildDefaultMetricsData(nanos),
			hecConfig:       defaultTestingHecConfig,
		},
		{
			name: "custom_hec",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = int64(13)
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				metrics := pdata.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
				attrs := resourceMetrics.Resource().Attributes()
				attrs.InsertString("myhost", "localhost")
				attrs.InsertString("mysource", "source")
				attrs.InsertString("mysourcetype", "sourcetype")
				attrs.InsertString("myindex", "index")

				metricPt := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("single")
				intPt := metricPt.Gauge().DataPoints().AppendEmpty()
				intPt.SetIntVal(13)
				intPt.Attributes().InsertString("k0", "v0")
				intPt.Attributes().InsertString("k1", "v1")
				intPt.Attributes().InsertString("k2", "v2")
				intPt.SetTimestamp(pdata.Timestamp(nanos))
				return metrics
			}(),
			hecConfig: &Config{HecToOtelAttrs: splunk.HecToOtelAttrs{
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Host:       "myhost",
			},
			},
		},
		{
			name: "double_counter",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = float64Ptr(13.13)
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("single")
				doublePt := metricPt.Gauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleVal(13.13)
				doublePt.Attributes().InsertString("k0", "v0")
				doublePt.Attributes().InsertString("k1", "v1")
				doublePt.Attributes().InsertString("k2", "v2")
				doublePt.SetTimestamp(pdata.Timestamp(nanos))
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "double_counter_as_string_pointer",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = strPtr("13.13")
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("single")
				doublePt := metricPt.Gauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleVal(13.13)
				doublePt.Attributes().InsertString("k0", "v0")
				doublePt.Attributes().InsertString("k1", "v1")
				doublePt.Attributes().InsertString("k2", "v2")
				doublePt.SetTimestamp(pdata.Timestamp(nanos))
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "double_counter_as_string",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = "13.13"
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetDataType(pdata.MetricDataTypeGauge)
				metricPt.SetName("single")
				doublePt := metricPt.Gauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleVal(13.13)
				doublePt.Attributes().InsertString("k0", "v0")
				doublePt.Attributes().InsertString("k1", "v1")
				doublePt.Attributes().InsertString("k2", "v2")
				doublePt.SetTimestamp(pdata.Timestamp(nanos))
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "zero_timestamp",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Time = new(float64)
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				return buildDefaultMetricsData(0)
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "empty_dimension_value",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["k0"] = ""
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				md := buildDefaultMetricsData(nanos)
				md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().UpdateString("k0", "")
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "invalid_point",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["metric_name:single"] = "foo"
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				return pdata.NewMetrics()
			}(),
			hecConfig:             defaultTestingHecConfig,
			wantDroppedTimeseries: 1,
		},
		{
			name: "cannot_convert_string",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				value := "foo"
				pt.Fields["metric_name:single"] = &value
				return pt
			}(),
			wantMetricsData: func() pdata.Metrics {
				return pdata.NewMetrics()
			}(),
			wantDroppedTimeseries: 1,
			hecConfig:             defaultTestingHecConfig,
		},
		{
			name: "nil_dimension_ignored",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Fields["k4"] = nil
				pt.Fields["k5"] = nil
				pt.Fields["k6"] = nil
				return pt
			}(),
			wantMetricsData: buildDefaultMetricsData(nanos),
			hecConfig:       defaultTestingHecConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, numDroppedTimeseries := splunkHecToMetricsData(zap.NewNop(), []*splunk.Event{tt.splunkDataPoint}, func(resource pdata.Resource) {}, tt.hecConfig)
			assert.Equal(t, tt.wantDroppedTimeseries, numDroppedTimeseries)
			assert.EqualValues(t, tt.wantMetricsData, sortMetricsAndLabels(md))
		})
	}
}

func buildDefaultMetricsData(time int64) pdata.Metrics {
	metrics := pdata.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	attrs := resourceMetrics.Resource().Attributes()
	attrs.InsertString("host.name", "localhost")
	attrs.InsertString("com.splunk.source", "source")
	attrs.InsertString("com.splunk.sourcetype", "sourcetype")
	attrs.InsertString("com.splunk.index", "index")

	metricPt := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metricPt.SetDataType(pdata.MetricDataTypeGauge)
	metricPt.SetName("single")
	intPt := metricPt.Gauge().DataPoints().AppendEmpty()
	intPt.SetIntVal(13)
	intPt.Attributes().InsertString("k0", "v0")
	intPt.Attributes().InsertString("k1", "v1")
	intPt.Attributes().InsertString("k2", "v2")
	intPt.SetTimestamp(pdata.Timestamp(time))
	return metrics
}

func strPtr(s string) *string {
	l := s
	return &l
}

func int64Ptr(i int64) *int64 {
	l := i
	return &l
}

func float64Ptr(f float64) *float64 {
	l := f
	return &l
}

func sortMetricsAndLabels(md pdata.Metrics) pdata.Metrics {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.InstrumentationLibraryMetrics().Len(); j++ {
			ilm := rm.InstrumentationLibraryMetrics().At(j)
			internalSortMetricsAndLabels(ilm.Metrics())
		}
	}
	return md
}

func internalSortMetricsAndLabels(metrics pdata.MetricSlice) {
	dest := pdata.NewMetricSlice()
	metricsMap := make(map[string]pdata.Metric)
	for k := 0; k < metrics.Len(); k++ {
		m := metrics.At(k)
		metricsMap[m.Name()] = m
		switch m.DataType() {
		case pdata.MetricDataTypeGauge:
			dps := m.Gauge().DataPoints()
			for l := 0; l < dps.Len(); l++ {
				dps.At(l).Attributes().Sort()
			}
		}
	}

	metricNames := make([]string, 0, len(metricsMap))
	for name := range metricsMap {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)
	for _, name := range metricNames {
		metricsMap[name].CopyTo(dest.AppendEmpty())
	}
	dest.CopyTo(metrics)
}
