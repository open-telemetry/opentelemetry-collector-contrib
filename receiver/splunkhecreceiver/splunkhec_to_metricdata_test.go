// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_splunkV2ToMetricsData(t *testing.T) {
	t.Parallel()
	// Timestamps for Splunk have a resolution to the millisecond, where the time is reported in seconds with a floating value to the millisecond.
	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	nanos := int64(sec * 1e9)

	buildDefaultSplunkDataPt := func() *splunk.Event {
		return &splunk.Event{
			Time:       sec,
			Host:       "localhost",
			Source:     "source",
			SourceType: "sourcetype",
			Index:      "index",
			Event:      "metrics",
			Fields: map[string]any{
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
		wantMetricsData       pmetric.Metrics
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
			wantMetricsData: func() pmetric.Metrics {
				metrics := buildDefaultMetricsData(nanos)
				mts := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

				metricPt := mts.AppendEmpty()
				metricPt.SetName("yetanother")
				intPt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				intPt.SetIntValue(14)
				intPt.SetTimestamp(pcommon.Timestamp(nanos))
				intPt.Attributes().PutStr("k0", "v0")
				intPt.Attributes().PutStr("k1", "v1")
				intPt.Attributes().PutStr("k2", "v2")

				metricPt2 := mts.AppendEmpty()
				metricPt2.SetName("yetanotherandanother")
				intPt2 := metricPt2.SetEmptyGauge().DataPoints().AppendEmpty()
				intPt2.SetIntValue(15)
				intPt2.SetTimestamp(pcommon.Timestamp(nanos))
				intPt2.Attributes().PutStr("k0", "v0")
				intPt2.Attributes().PutStr("k1", "v1")
				intPt2.Attributes().PutStr("k2", "v2")

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
			wantMetricsData: func() pmetric.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetName("single")
				doublePt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleValue(13.13)
				doublePt.SetTimestamp(pcommon.Timestamp(nanos))
				doublePt.Attributes().PutStr("k0", "v0")
				doublePt.Attributes().PutStr("k1", "v1")
				doublePt.Attributes().PutStr("k2", "v2")
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
			wantMetricsData: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
				attrs := resourceMetrics.Resource().Attributes()
				attrs.PutStr("myhost", "localhost")
				attrs.PutStr("mysource", "source")
				attrs.PutStr("mysourcetype", "sourcetype")
				attrs.PutStr("myindex", "index")

				metricPt := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metricPt.SetName("single")
				intPt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				intPt.SetIntValue(13)
				intPt.Attributes().PutStr("k0", "v0")
				intPt.Attributes().PutStr("k1", "v1")
				intPt.Attributes().PutStr("k2", "v2")
				intPt.SetTimestamp(pcommon.Timestamp(nanos))
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
			wantMetricsData: func() pmetric.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetName("single")
				doublePt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleValue(13.13)
				doublePt.Attributes().PutStr("k0", "v0")
				doublePt.Attributes().PutStr("k1", "v1")
				doublePt.Attributes().PutStr("k2", "v2")
				doublePt.SetTimestamp(pcommon.Timestamp(nanos))
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
			wantMetricsData: func() pmetric.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetName("single")
				doublePt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleValue(13.13)
				doublePt.Attributes().PutStr("k0", "v0")
				doublePt.Attributes().PutStr("k1", "v1")
				doublePt.Attributes().PutStr("k2", "v2")
				doublePt.SetTimestamp(pcommon.Timestamp(nanos))
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
			wantMetricsData: func() pmetric.Metrics {
				md := buildDefaultMetricsData(nanos)
				mts := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				metricPt := mts.At(0)
				metricPt.SetName("single")
				doublePt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
				doublePt.SetDoubleValue(13.13)
				doublePt.Attributes().PutStr("k0", "v0")
				doublePt.Attributes().PutStr("k1", "v1")
				doublePt.Attributes().PutStr("k2", "v2")
				doublePt.SetTimestamp(pcommon.Timestamp(nanos))
				return md
			}(),
			hecConfig: defaultTestingHecConfig,
		},
		{
			name: "zero_timestamp",
			splunkDataPoint: func() *splunk.Event {
				pt := buildDefaultSplunkDataPt()
				pt.Time = 0
				return pt
			}(),
			wantMetricsData: func() pmetric.Metrics {
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
			wantMetricsData: func() pmetric.Metrics {
				md := buildDefaultMetricsData(nanos)
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().PutStr("k0", "")
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
			wantMetricsData:       pmetric.NewMetrics(),
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
			wantMetricsData:       pmetric.NewMetrics(),
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
			md, numDroppedTimeseries := splunkHecToMetricsData(zap.NewNop(), []*splunk.Event{tt.splunkDataPoint}, func(resource pcommon.Resource) {}, tt.hecConfig)
			assert.Equal(t, tt.wantDroppedTimeseries, numDroppedTimeseries)
			assert.NoError(t, pmetrictest.CompareMetrics(tt.wantMetricsData, md, pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

func TestGroupMetricsByResource(t *testing.T) {
	t.Parallel()
	// Timestamps for Splunk have a resolution to the millisecond, where the time is reported in seconds with a floating value to the millisecond.
	now := time.Now()
	msecInt64 := now.UnixNano() / 1e6
	sec := float64(msecInt64) / 1e3
	nanoseconds := int64(sec * 1e9)
	events := []*splunk.Event{
		{
			Time:       sec,
			Host:       "1",
			Source:     "1",
			SourceType: "1",
			Index:      "1",
			Fields: map[string]any{
				"field":          "value1",
				"metric_name:m1": int64(1),
			},
		},
		{
			Time:       sec,
			Host:       "2",
			Source:     "2",
			SourceType: "2",
			Index:      "2",
			Fields: map[string]any{
				"field":          "value2",
				"metric_name:m2": int64(2),
			},
		},
		{
			Time:       sec,
			Host:       "1",
			Source:     "1",
			SourceType: "1",
			Index:      "1",
			Fields: map[string]any{
				"field":          "value1",
				"metric_name:m1": int64(3),
			},
		},
		{
			Time:       sec,
			Host:       "2",
			Source:     "2",
			SourceType: "2",
			Index:      "2",
			Event:      "Event-4",
			Fields: map[string]any{
				"field":          "value2",
				"metric_name:m2": int64(4),
			},
		},
		{
			Time:       sec,
			Host:       "1",
			Source:     "2",
			SourceType: "1",
			Index:      "2",
			Fields: map[string]any{
				"field":           "value1-2",
				"metric_name:m12": int64(5),
			},
		},
		{
			Time:       sec,
			Host:       "2",
			Source:     "1",
			SourceType: "2",
			Index:      "1",
			Event:      "Event-6",
			Fields: map[string]any{
				"field":           "value2-1",
				"metric_name:m21": int64(6),
			},
		},
	}
	metrics := pmetric.NewMetrics()
	{
		mr := metrics.ResourceMetrics().AppendEmpty()
		updateResourceMap(mr.Resource().Attributes(), "1", "1", "1", "1")
		sm := mr.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("m1")
		dataPt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(1)

		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value1")

		metric = sm.Metrics().AppendEmpty()
		metric.SetName("m1")
		dataPt = metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(3)

		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value1")
	}
	{
		mr := metrics.ResourceMetrics().AppendEmpty()
		updateResourceMap(mr.Resource().Attributes(), "2", "2", "2", "2")
		sm := mr.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("m2")
		dataPt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(2)

		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value2")

		metric = sm.Metrics().AppendEmpty()
		metric.SetName("m2")
		dataPt = metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(4)

		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value2")
	}
	{
		mr := metrics.ResourceMetrics().AppendEmpty()
		updateResourceMap(mr.Resource().Attributes(), "1", "2", "1", "2")
		sm := mr.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("m12")
		dataPt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(5)
		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value1-2")
	}
	{
		mr := metrics.ResourceMetrics().AppendEmpty()
		updateResourceMap(mr.Resource().Attributes(), "2", "1", "2", "1")
		sm := mr.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("m21")
		dataPt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPt.SetIntValue(6)
		dataPt.SetTimestamp(pcommon.Timestamp(nanoseconds))
		dataPt.Attributes().PutStr("field", "value2-1")
	}
	md, numDroppedTimeseries := splunkHecToMetricsData(zap.NewNop(), events, func(resource pcommon.Resource) {}, defaultTestingHecConfig)
	assert.Equal(t, 0, numDroppedTimeseries)
	assert.EqualValues(t, metrics, md)
}

func buildDefaultMetricsData(time int64) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	attrs := resourceMetrics.Resource().Attributes()
	attrs.PutStr("host.name", "localhost")
	attrs.PutStr("com.splunk.source", "source")
	attrs.PutStr("com.splunk.sourcetype", "sourcetype")
	attrs.PutStr("com.splunk.index", "index")

	metricPt := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metricPt.SetName("single")
	intPt := metricPt.SetEmptyGauge().DataPoints().AppendEmpty()
	intPt.SetIntValue(13)
	intPt.Attributes().PutStr("k0", "v0")
	intPt.Attributes().PutStr("k1", "v1")
	intPt.Attributes().PutStr("k2", "v2")
	intPt.SetTimestamp(pcommon.Timestamp(time))
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
