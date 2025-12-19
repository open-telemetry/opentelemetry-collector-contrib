// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func Test_metricDataToSplunk(t *testing.T) {
	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tsMSecs := timestampToSecondsWithMillisecondPrecision(ts)

	doubleVal := 1234.5678
	int64Val := int64(123)

	distributionBounds := []float64{1, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 5}

	tests := []struct {
		name              string
		resourceFn        func() pcommon.Resource
		metricsDataFn     func() pmetric.Metric
		wantSplunkMetrics []*Event
		toOtelAttrs       HecToOtelAttrs
		source            string
		sourceType        string
		index             string
	}{
		{
			name:       "nil_gauge_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetEmptyGauge()
				return gauge
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "nan_gauge_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("gauge_with_dims")
				dp := gauge.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetDoubleValue(math.NaN())
				return gauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, "NaN", "", "", "", "unknown"),
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "+Inf_gauge_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("gauge_with_dims")
				dp := gauge.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetDoubleValue(math.Inf(1))
				return gauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, "+Inf", "", "", "", "unknown"),
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "-Inf_gauge_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("gauge_with_dims")
				dp := gauge.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.SetDoubleValue(math.Inf(-1))
				return gauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, "-Inf", "", "", "", "unknown"),
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "nil_histogram_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("histogram_with_dims")
				histogram.SetEmptyHistogram()
				return histogram
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "nil_sum_value",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				sum := pmetric.NewMetric()
				sum.SetName("sum_with_dims")
				sum.SetEmptySum()
				return sum
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "gauge_empty_data_point",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				gauge := pmetric.NewMetric()
				gauge.SetName("gauge_with_dims")
				gauge.SetEmptyGauge().DataPoints().AppendEmpty()
				return gauge
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "histogram_empty_data_point",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("histogram_with_dims")
				histogram.SetEmptyHistogram().DataPoints().AppendEmpty()
				return histogram
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "sum_empty_data_point",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				sum := pmetric.NewMetric()
				sum.SetName("sum_with_dims")
				sum.SetEmptySum().DataPoints().AppendEmpty()
				return sum
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name: "int_gauge",
			resourceFn: func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("com.splunk.source", "mysource")
				res.Attributes().PutStr("host.name", "myhost")
				res.Attributes().PutStr("com.splunk.sourcetype", "mysourcetype")
				res.Attributes().PutStr("com.splunk.index", "myindex")
				res.Attributes().PutStr("k0", "v0")
				res.Attributes().PutStr("k1", "v1")
				return res
			},
			metricsDataFn: func() pmetric.Metric {
				intGauge := pmetric.NewMetric()
				intGauge.SetName("gauge_int_with_dims")
				intDataPt := intGauge.SetEmptyGauge().DataPoints().AppendEmpty()
				intDataPt.SetIntValue(int64Val)
				intDataPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				intDataPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))

				return intGauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_int_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, int64Val, "mysource", "mysourcetype", "myindex", "myhost"),
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},

		{
			name: "double_gauge",
			resourceFn: func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("com.splunk.source", "mysource")
				res.Attributes().PutStr("host.name", "myhost")
				res.Attributes().PutStr("com.splunk.sourcetype", "mysourcetype")
				res.Attributes().PutStr("com.splunk.index", "myindex")
				res.Attributes().PutStr("k0", "v0")
				res.Attributes().PutStr("k1", "v1")
				return res
			},
			metricsDataFn: func() pmetric.Metric {
				doubleGauge := pmetric.NewMetric()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleDataPt := doubleGauge.SetEmptyGauge().DataPoints().AppendEmpty()
				doubleDataPt.SetDoubleValue(doubleVal)
				doubleDataPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))

				return doubleGauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, doubleVal, "mysource", "mysourcetype", "myindex", "myhost"),
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},

		{
			name:       "histogram_no_upper_bound",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("double_histogram_with_dims")
				histogramPt := histogram.SetEmptyHistogram().DataPoints().AppendEmpty()
				histogramPt.ExplicitBounds().FromRaw(distributionBounds)
				histogramPt.BucketCounts().FromRaw([]uint64{4, 2, 3})
				histogramPt.SetSum(23)
				histogramPt.SetCount(7)
				histogramPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				return histogram
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "histogram",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				histogram := pmetric.NewMetric()
				histogram.SetName("double_histogram_with_dims")
				histogramPt := histogram.SetEmptyHistogram().DataPoints().AppendEmpty()
				histogramPt.ExplicitBounds().FromRaw(distributionBounds)
				histogramPt.BucketCounts().FromRaw(distributionCounts)
				histogramPt.SetSum(23)
				histogramPt.SetCount(7)
				histogramPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				return histogram
			},
			wantSplunkMetrics: []*Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
						"k0": "v0",
						"k1": "v1",
						"le": "+Inf",
						"metric_name:double_histogram_with_dims_bucket": uint64(14),
						"metric_type": "Histogram",
					},
				},
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},

		{
			name:       "int_sum",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				intSum := pmetric.NewMetric()
				intSum.SetName("int_sum_with_dims")
				intDataPt := intSum.SetEmptySum().DataPoints().AppendEmpty()
				intDataPt.SetTimestamp(ts)
				intDataPt.SetIntValue(62)
				return intSum
			},
			wantSplunkMetrics: []*Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]any{
						"k0":                            "v0",
						"k1":                            "v1",
						"metric_name:int_sum_with_dims": int64(62),
						"metric_type":                   "Sum",
					},
				},
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "double_sum",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				doubleSum := pmetric.NewMetric()
				doubleSum.SetName("double_sum_with_dims")
				doubleDataPt := doubleSum.SetEmptySum().DataPoints().AppendEmpty()
				doubleDataPt.SetTimestamp(ts)
				doubleDataPt.SetDoubleValue(62)
				return doubleSum
			},
			wantSplunkMetrics: []*Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]any{
						"k0":                               "v0",
						"k1":                               "v1",
						"metric_name:double_sum_with_dims": float64(62),
						"metric_type":                      "Sum",
					},
				},
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "summary",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				summary := pmetric.NewMetric()
				summary.SetName("summary")
				summaryPt := summary.SetEmptySummary().DataPoints().AppendEmpty()
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
			wantSplunkMetrics: []*Event{
				{
					Host:       "unknown",
					Source:     "",
					SourceType: "",
					Event:      "metric",
					Time:       tsMSecs,
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
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
					Fields: map[string]any{
						"k0":                      "v0",
						"k1":                      "v1",
						"qt":                      "0.6",
						"metric_name:summary_0.6": float64(45),
						"metric_type":             "Summary",
					},
				},
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			source:      "",
			sourceType:  "",
			index:       "",
		},
		{
			name:       "unknown_type",
			resourceFn: newMetricsWithResources,
			metricsDataFn: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("unknown_with_dims")
				return metric
			},
			wantSplunkMetrics: nil,
			toOtelAttrs:       DefaultHecToOtelAttrs(),
			source:            "",
			sourceType:        "",
			index:             "",
		},

		{
			name: "custom_config_mapping",
			resourceFn: func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("mysource", "mysource2")
				res.Attributes().PutStr("myhost", "myhost2")
				res.Attributes().PutStr("mysourcetype", "mysourcetype2")
				res.Attributes().PutStr("myindex", "myindex2")
				res.Attributes().PutStr("k0", "v0")
				res.Attributes().PutStr("k1", "v1")
				return res
			},
			metricsDataFn: func() pmetric.Metric {
				doubleGauge := pmetric.NewMetric()
				doubleGauge.SetName("gauge_double_with_dims")
				doubleDataPt := doubleGauge.SetEmptyGauge().DataPoints().AppendEmpty()
				doubleDataPt.SetDoubleValue(doubleVal)
				doubleDataPt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))

				return doubleGauge
			},
			wantSplunkMetrics: []*Event{
				commonSplunkMetric("gauge_double_with_dims", tsMSecs, []string{"k0", "k1", "metric_type"}, []any{"v0", "v1", "Gauge"}, doubleVal, "mysource2", "mysourcetype2", "myindex2", "myhost2"),
			},
			toOtelAttrs: HecToOtelAttrs{
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Host:       "myhost",
			},
			source:     "",
			sourceType: "",
			index:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.resourceFn()
			md := tt.metricsDataFn()
			gotMetrics := MetricToSplunkEvent(res, md, zap.NewNop(), tt.toOtelAttrs, tt.source, tt.sourceType, tt.index)
			encoder := json.NewEncoder(io.Discard)
			for i, want := range tt.wantSplunkMetrics {
				assert.Equal(t, want, gotMetrics[i])
				err := encoder.Encode(gotMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func Test_mergeEventsToMultiMetricFormat(t *testing.T) {
	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tests := []struct {
		name   string
		events []*Event
		merged []*Event
	}{
		{
			name:   "no events",
			events: []*Event{},
			merged: []*Event{},
		},
		{
			name: "two events that can merge",
			events: []*Event{
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 123,
				}),
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":                  "bar",
					"metric_name:othermem": 1233.4,
				}),
			},
			merged: []*Event{
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":                  "bar",
					"metric_name:mem":      123,
					"metric_name:othermem": 1233.4,
				}),
			},
		},
		{
			name: "two events that cannot merge",
			events: []*Event{
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 123,
				}),
				createEvent(ts, "host2", "source", "sourcetype", "index", map[string]any{
					"foo":                  "bar",
					"metric_name:othermem": 1233.4,
				}),
			},
			merged: []*Event{
				createEvent(ts, "host2", "source", "sourcetype", "index", map[string]any{
					"foo":                  "bar",
					"metric_name:othermem": 1233.4,
				}),
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 123,
				}),
			},
		},
		{
			name: "two events with the same fields, but different metric value, last value wins",
			events: []*Event{
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 123,
				}),
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 1233.4,
				}),
			},
			merged: []*Event{
				createEvent(ts, "host", "source", "sourcetype", "index", map[string]any{
					"foo":             "bar",
					"metric_name:mem": 1233.4,
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := MergeEventsToMultiMetricFormat(tt.events)
			assert.NoError(t, err)
			assert.Len(t, merged, len(tt.merged))
			for _, want := range tt.merged {
				found := false
				for _, m := range merged {
					if assert.ObjectsAreEqual(want, m) {
						found = true
						break
					}
				}
				assert.Truef(t, found, "Event not found: %v", want)
			}
		})
	}
}

func commonSplunkMetric(
	metricName string,
	ts float64,
	keys []string,
	values []any,
	val any,
	source string,
	sourcetype string,
	index string,
	host string,
) *Event {
	fields := map[string]any{fmt.Sprintf("metric_name:%s", metricName): val}

	for i, k := range keys {
		fields[k] = values[i]
	}

	return &Event{
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
	ts := pcommon.Timestamp(32001000345)
	assert.Equal(t, 32.001, timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRounding(t *testing.T) {
	ts := pcommon.Timestamp(32001999345)
	assert.Equal(t, 32.002, timestampToSecondsWithMillisecondPrecision(ts))
}

func TestTimestampFormatRoundingWithNanos(t *testing.T) {
	ts := pcommon.Timestamp(9999999999991500001)
	assert.Equal(t, 9999999999.992, timestampToSecondsWithMillisecondPrecision(ts))
}

func TestNilTimeWhenTimestampIsZero(t *testing.T) {
	ts := pcommon.Timestamp(0)
	assert.Zero(t, timestampToSecondsWithMillisecondPrecision(ts))
}

func TestMergeEvents(t *testing.T) {
	json1 := `{"event":"metric","fields":{"IF-Azure":"azure-env","k8s.cluster.name":"devops-uat","k8s.namespace.name":"splunk-collector-tests","k8s.node.name":"myk8snodename","k8s.pod.name":"my-otel-collector-pod","metric_type":"Gauge","metricsIndex":"test_metrics","metricsPlatform":"unset","resourceAttrs":"NO","testNumber":"number42","testRun":"42","metric_name:otel.collector.test":3411}}`
	json2 := `{"event":"metric","fields":{"IF-Azure":"azure-env","k8s.cluster.name":"devops-uat","k8s.namespace.name":"splunk-collector-tests","k8s.node.name":"myk8snodename","k8s.pod.name":"my-otel-collector-pod","metric_type":"Gauge","metricsIndex":"test_metrics","metricsPlatform":"unset","resourceAttrs":"NO","testNumber":"number42","testRun":"42","metric_name:otel.collector.test2":26059}}`
	ev1 := &Event{}
	err := json.Unmarshal([]byte(json1), ev1)
	require.NoError(t, err)
	ev2 := &Event{}
	err = json.Unmarshal([]byte(json2), ev2)
	require.NoError(t, err)
	events := []*Event{ev1, ev2}
	merged, err := MergeEventsToMultiMetricFormat(events)
	require.NoError(t, err)
	require.Len(t, merged, 1)
	b, err := json.Marshal(merged[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"host":"","event":"metric","fields":{"IF-Azure":"azure-env","k8s.cluster.name":"devops-uat","k8s.namespace.name":"splunk-collector-tests","k8s.node.name":"myk8snodename","k8s.pod.name":"my-otel-collector-pod","metric_name:otel.collector.test":3411,"metric_name:otel.collector.test2":26059,"metric_type":"Gauge","metricsIndex":"test_metrics","metricsPlatform":"unset","resourceAttrs":"NO","testNumber":"number42","testRun":"42"}}`, string(b))
}

func newMetricsWithResources() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().PutStr("k0", "v0")
	res.Attributes().PutStr("k1", "v1")
	return res
}
