// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestAddResourceTargetInfoV2(t *testing.T) {
	resourceAttrMap := map[string]any{
		"service.name":        "service-name",
		"service.namespace":   "service-namespace",
		"service.instance.id": "service-instance-id",
	}
	resourceWithServiceAttrs := pcommon.NewResource()
	require.NoError(t, resourceWithServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	resourceWithServiceAttrs.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	resourceWithOnlyServiceAttrs := pcommon.NewResource()
	require.NoError(t, resourceWithOnlyServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	// service.name is an identifying resource attribute.
	resourceWithOnlyServiceName := pcommon.NewResource()
	resourceWithOnlyServiceName.Attributes().PutStr("service.name", "service-name")
	resourceWithOnlyServiceName.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	// service.instance.id is an identifying resource attribute.
	resourceWithOnlyServiceID := pcommon.NewResource()
	resourceWithOnlyServiceID.Attributes().PutStr("service.instance.id", "service-instance-id")
	resourceWithOnlyServiceID.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	for _, tc := range []struct {
		desc           string
		resource       pcommon.Resource
		settings       Settings
		timestamp      pcommon.Timestamp
		wantLabels     []prompb.Label
		wantLabelsRefs []uint32
		wantHelpRef    uint32
	}{
		{
			desc:     "empty resource",
			resource: pcommon.NewResource(),
		},
		{
			desc:     "disable target info metric",
			resource: resourceWithOnlyServiceName,
			settings: Settings{DisableTargetInfo: true},
		},
		{
			desc:      "with resource missing both service.name and service.instance.id resource attributes",
			resource:  testdata.GenerateMetricsNoLibraries().ResourceMetrics().At(0).Resource(),
			timestamp: testdata.TestMetricStartTimestamp,
		},
		{
			desc:      "with resource including service.instance.id, and missing service.name resource attribute",
			resource:  resourceWithOnlyServiceID,
			timestamp: testdata.TestMetricStartTimestamp,
			wantLabels: []prompb.Label{
				{Name: model.MetricNameLabel, Value: "target_info"},
				{Name: model.InstanceLabel, Value: "service-instance-id"},
				{Name: "resource_attr", Value: "resource-attr-val-1"},
			},
			wantLabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
			wantHelpRef:    7,
		},
		{
			desc:      "with resource including service.name, and missing service.instance.id resource attribute",
			resource:  resourceWithOnlyServiceName,
			timestamp: testdata.TestMetricStartTimestamp,
			wantLabels: []prompb.Label{
				{Name: model.MetricNameLabel, Value: "target_info"},
				{Name: model.JobLabel, Value: "service-name"},
				{Name: "resource_attr", Value: "resource-attr-val-1"},
			},
			wantLabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
			wantHelpRef:    7,
		},
		{
			desc:      "with valid resource, with namespace",
			resource:  resourceWithOnlyServiceName,
			timestamp: testdata.TestMetricStartTimestamp,
			settings:  Settings{Namespace: "foo"},
			wantLabels: []prompb.Label{
				{Name: model.MetricNameLabel, Value: "foo_target_info"},
				{Name: model.JobLabel, Value: "service-name"},
				{Name: "resource_attr", Value: "resource-attr-val-1"},
			},
			wantLabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
			wantHelpRef:    7,
		},
		{
			desc:      "with resource, with service attributes",
			resource:  resourceWithServiceAttrs,
			timestamp: testdata.TestMetricStartTimestamp,
			wantLabels: []prompb.Label{
				{Name: model.MetricNameLabel, Value: "target_info"},
				{Name: model.InstanceLabel, Value: "service-instance-id"},
				{Name: model.JobLabel, Value: "service-namespace/service-name"},
				{Name: "resource_attr", Value: "resource-attr-val-1"},
			},
			wantLabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
			wantHelpRef:    9,
		},
		{
			desc:      "with resource, with only service attributes",
			resource:  resourceWithOnlyServiceAttrs,
			timestamp: testdata.TestMetricStartTimestamp,
		},
		{
			// If there's no timestamp, target_info shouldn't be generated, since we don't know when the write is from.
			desc:      "with resource, with service attributes, without timestamp",
			resource:  resourceWithServiceAttrs,
			timestamp: 0,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			converter := newPrometheusConverterV2(Settings{})

			err := converter.addResourceTargetInfoV2(tc.resource, tc.settings, tc.timestamp)
			require.NoError(t, err)

			if len(tc.wantLabels) == 0 || tc.settings.DisableTargetInfo {
				assert.Empty(t, converter.timeSeries())
				return
			}

			expected := map[uint64]*writev2.TimeSeries{
				timeSeriesSignature(tc.wantLabels): {
					LabelsRefs: tc.wantLabelsRefs,
					Samples: []writev2.Sample{
						{
							Value:     1,
							Timestamp: 1581452772000,
						},
					},
					Metadata: writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
						HelpRef: tc.wantHelpRef,
						UnitRef: 0,
					},
				},
			}
			assert.Exactly(t, expected, converter.unique)
			assert.Empty(t, converter.conflicts)
		})
	}
}

func TestPrometheusConverterV2_AddSummaryDataPoints(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[uint64]*writev2.TimeSeries
	}{
		{
			name: "summary with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_summary")
				metric.SetEmptySummary()

				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + countStr},
				}
				sumLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + sumStr},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 3},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_SUMMARY,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(sumLabels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_SUMMARY,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "summary without start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_summary")
				metric.SetEmptySummary()

				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + countStr},
				}
				sumLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + sumStr},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 3},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_SUMMARY,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(sumLabels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_SUMMARY,
							HelpRef: 0,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()
			converter := newPrometheusConverterV2(Settings{})

			unitNamer := otlptranslator.UnitNamer{}
			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: unitNamer.Build(metric.Unit()),
			}

			err := converter.addSummaryDataPoints(
				metric.Summary().DataPoints(),
				pcommon.NewResource(),
				pcommon.NewInstrumentationScope(),
				Settings{},
				metric.Name(),
				m,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want(), converter.unique)
			assert.Empty(t, converter.conflicts)
		})
	}
}

func TestPrometheusConverterV2_AddHistogramDataPoints(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[uint64]*writev2.TimeSeries
	}{
		{
			name: "histogram with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)
				pt.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(infLabels): {
						LabelsRefs: []uint32{1, 3, 4, 5},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "histogram without start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(infLabels): {
						LabelsRefs: []uint32{1, 3, 4, 5},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()
			converter := newPrometheusConverterV2(Settings{})
			unitNamer := otlptranslator.UnitNamer{}
			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: unitNamer.Build(metric.Unit()),
			}
			err := converter.addHistogramDataPoints(
				metric.Histogram().DataPoints(),
				pcommon.NewResource(),
				pcommon.NewInstrumentationScope(),
				Settings{},
				metric.Name(),
				m,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want(), converter.unique)
			assert.Empty(t, converter.conflicts)
		})
	}
}

func TestPrometheusConverterV2_AddSampleWithLabels(t *testing.T) {
	tests := []struct {
		name            string
		sampleValue     float64
		timestamp       int64
		noRecordedValue bool
		baseName        string
		baseLabels      []prompb.Label
		labelName       string
		labelValue      string
		metadata        metadata
		want            func() map[uint64]*writev2.TimeSeries
	}{
		{
			name:        "normal sample with additional label",
			sampleValue: 42.5,
			timestamp:   1234567890000,
			baseName:    "test_metric",
			baseLabels: []prompb.Label{
				{Name: "base_label", Value: "base_value"},
			},
			labelName:  "extra_label",
			labelValue: "extra_value",
			metadata: metadata{
				Type: writev2.Metadata_METRIC_TYPE_GAUGE,
				Help: "Test metric",
				Unit: "bytes",
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: "base_label", Value: "base_value"},
					{Name: "extra_label", Value: "extra_value"},
					{Name: model.MetricNameLabel, Value: "test_metric"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
						Samples: []writev2.Sample{
							{Value: 42.5, Timestamp: 1234567890000},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 7,
							UnitRef: 8,
						},
					},
				}
			},
		},
		{
			name:        "normal sample without additional label",
			sampleValue: 100.0,
			timestamp:   1234567890000,
			baseName:    "test_metric_no_extra",
			baseLabels: []prompb.Label{
				{Name: "base_label", Value: "base_value"},
			},
			labelName:  "",
			labelValue: "",
			metadata: metadata{
				Type: writev2.Metadata_METRIC_TYPE_COUNTER,
				Help: "Test counter",
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: "base_label", Value: "base_value"},
					{Name: model.MetricNameLabel, Value: "test_metric_no_extra"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4},
						Samples: []writev2.Sample{
							{Value: 100.0, Timestamp: 1234567890000},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
							HelpRef: 5,
							UnitRef: 0,
						},
					},
				}
			},
		},
		{
			name:            "stale sample with noRecordedValue true",
			sampleValue:     50.0,
			timestamp:       1234567890000,
			noRecordedValue: true,
			baseName:        "test_stale_metric",
			baseLabels: []prompb.Label{
				{Name: "instance", Value: "localhost:8080"},
			},
			labelName:  "job",
			labelValue: "test_job",
			metadata: metadata{
				Type: writev2.Metadata_METRIC_TYPE_GAUGE,
				Help: "Test stale metric",
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: "instance", Value: "localhost:8080"},
					{Name: "job", Value: "test_job"},
					{Name: model.MetricNameLabel, Value: "test_stale_metric"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
						Samples: []writev2.Sample{
							{Value: math.Float64frombits(value.StaleNaN), Timestamp: 1234567890000},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 7,
							UnitRef: 0,
						},
					},
				}
			},
		},
		{
			name:        "empty base labels",
			sampleValue: 15.0,
			timestamp:   1234567890000,
			baseName:    "simple_metric",
			baseLabels:  []prompb.Label{},
			labelName:   "service",
			labelValue:  "my_service",
			metadata: metadata{
				Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
				Help: "Simple histogram",
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: "service", Value: "my_service"},
					{Name: model.MetricNameLabel, Value: "simple_metric"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4},
						Samples: []writev2.Sample{
							{Value: 15.0, Timestamp: 1234567890000},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 5,
							UnitRef: 0,
						},
					},
				}
			},
		},
		{
			name:        "partial label name only",
			sampleValue: 25.5,
			timestamp:   1234567890000,
			baseName:    "partial_metric",
			baseLabels: []prompb.Label{
				{Name: "region", Value: "us-west-1"},
			},
			labelName:  "environment",
			labelValue: "", // empty value should not add the label
			metadata: metadata{
				Type: writev2.Metadata_METRIC_TYPE_SUMMARY,
				Help: "Partial label test",
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: "region", Value: "us-west-1"},
					{Name: model.MetricNameLabel, Value: "partial_metric"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4},
						Samples: []writev2.Sample{
							{Value: 25.5, Timestamp: 1234567890000},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_SUMMARY,
							HelpRef: 5,
							UnitRef: 0,
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := newPrometheusConverterV2(Settings{})

			converter.addSampleWithLabels(
				tt.sampleValue,
				tt.timestamp,
				tt.noRecordedValue,
				tt.baseName,
				tt.baseLabels,
				tt.labelName,
				tt.labelValue,
				tt.metadata,
			)

			w := tt.want()
			diff := cmp.Diff(w, converter.unique, cmpopts.EquateNaNs())
			assert.Empty(t, diff)
			assert.Empty(t, converter.conflicts)
		})
	}
}
