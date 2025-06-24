// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestAddResourceTargetInfoV2(t *testing.T) {
	resourceAttrMap := map[string]any{
		string(conventions.ServiceNameKey):       "service-name",
		string(conventions.ServiceNamespaceKey):  "service-namespace",
		string(conventions.ServiceInstanceIDKey): "service-instance-id",
	}
	resourceWithServiceAttrs := pcommon.NewResource()
	require.NoError(t, resourceWithServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	resourceWithServiceAttrs.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	resourceWithOnlyServiceAttrs := pcommon.NewResource()
	require.NoError(t, resourceWithOnlyServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	// service.name is an identifying resource attribute.
	resourceWithOnlyServiceName := pcommon.NewResource()
	resourceWithOnlyServiceName.Attributes().PutStr(string(conventions.ServiceNameKey), "service-name")
	resourceWithOnlyServiceName.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	// service.instance.id is an identifying resource attribute.
	resourceWithOnlyServiceID := pcommon.NewResource()
	resourceWithOnlyServiceID.Attributes().PutStr(string(conventions.ServiceInstanceIDKey), "service-instance-id")
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
			converter := newPrometheusConverterV2()

			converter.addResourceTargetInfoV2(tc.resource, tc.settings, tc.timestamp)

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
			// TODO check when conflicts handling is implemented
			// assert.Empty(t, converter.conflicts)
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
			converter := newPrometheusConverterV2()

			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: prometheustranslator.BuildCompliantPrometheusUnit(metric.Unit()),
			}

			converter.addSummaryDataPoints(
				metric.Summary().DataPoints(),
				pcommon.NewResource(),
				Settings{},
				metric.Name(),
				m,
			)

			assert.Equal(t, tt.want(), converter.unique)
			// TODO check when conflicts handling is implemented
			// assert.Empty(t, converter.conflicts)
		})
	}
}
