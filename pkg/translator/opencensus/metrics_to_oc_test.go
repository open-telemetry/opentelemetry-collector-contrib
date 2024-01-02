// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"testing"
	"time"

	occommon "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	ocmetrics "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	ocresource "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestMetricsToOC(t *testing.T) {
	sampleMetricData := testdata.GeneratMetricsAllTypesWithSampleDatapoints()
	attrs := sampleMetricData.ResourceMetrics().At(0).Resource().Attributes()
	attrs.PutStr(conventions.AttributeHostName, "host1")
	attrs.PutInt(conventions.AttributeProcessPID, 123)
	attrs.PutStr(occonventions.AttributeProcessStartTime, "2020-02-11T20:26:00Z")
	attrs.PutStr(conventions.AttributeTelemetrySDKLanguage, "cpp")
	attrs.PutStr(conventions.AttributeTelemetrySDKVersion, "v2.0.1")
	attrs.PutStr(occonventions.AttributeExporterVersion, "v1.2.0")

	tests := []struct {
		name     string
		internal pmetric.Metrics
		oc       *agentmetricspb.ExportMetricsServiceRequest
	}{
		{
			name:     "one-empty-resource-metrics",
			internal: testdata.GenerateMetricsOneEmptyResourceMetrics(),
			oc:       &agentmetricspb.ExportMetricsServiceRequest{},
		},

		{
			name:     "no-libraries",
			internal: testdata.GenerateMetricsNoLibraries(),
			oc:       generateOCTestDataNoMetrics(),
		},

		{
			name:     "one-empty-instrumentation-library",
			internal: testdata.GenerateMetricsOneEmptyInstrumentationLibrary(),
			oc:       generateOCTestDataNoMetrics(),
		},

		{
			name:     "one-metric-no-resource",
			internal: testdata.GenerateMetricsOneMetricNoResource(),
			oc: &agentmetricspb.ExportMetricsServiceRequest{
				Metrics: []*ocmetrics.Metric{
					generateOCTestMetricCumulativeInt(),
				},
			},
		},

		{
			name:     "one-metric",
			internal: testdata.GenerateMetricsOneMetric(),
			oc:       generateOCTestDataMetricsOneMetric(),
		},

		{
			name:     "one-metric-no-labels",
			internal: testdata.GenerateMetricsOneMetricNoAttributes(),
			oc:       generateOCTestDataNoLabels(),
		},

		// TODO: Enable this after the testdata.GenerateMetricsAllTypesNoDataPoints is changed
		//  to generate one sum and one gauge (no difference between int/double when no points).
		// {
		//   name:     "all-types-no-data-points",
		//   internal: testdata.GenerateMetricsAllTypesNoDataPoints(),
		//   oc:       generateOCTestDataNoPoints(),
		// },

		{
			name:     "all-types",
			internal: sampleMetricData,
			oc:       generateOCTestData(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotNode, gotResource, gotMetrics := ResourceMetricsToOC(test.internal.ResourceMetrics().At(0))
			assert.EqualValues(t, test.oc.Node, gotNode)
			assert.EqualValues(t, test.oc.Resource, gotResource)
			assert.EqualValues(t, test.oc.Metrics, gotMetrics)
		})
	}
}

func TestMetricsToOC_InvalidDataType(t *testing.T) {
	internal := testdata.GenerateMetricsMetricTypeInvalid()
	want := &agentmetricspb.ExportMetricsServiceRequest{
		Node: &occommon.Node{},
		Resource: &ocresource.Resource{
			Labels: map[string]string{"resource-attr": "resource-attr-val-1"},
		},
		Metrics: []*ocmetrics.Metric{
			{
				MetricDescriptor: &ocmetrics.MetricDescriptor{
					Name:      testdata.TestSumIntMetricName,
					Unit:      "1",
					Type:      ocmetrics.MetricDescriptor_UNSPECIFIED,
					LabelKeys: nil,
				},
			},
		},
	}
	gotNode, gotResource, gotMetrics := ResourceMetricsToOC(internal.ResourceMetrics().At(0))
	assert.EqualValues(t, want.Node, gotNode)
	assert.EqualValues(t, want.Resource, gotResource)
	assert.EqualValues(t, want.Metrics, gotMetrics)
}

func generateOCTestData() *agentmetricspb.ExportMetricsServiceRequest {
	ts := timestamppb.New(time.Date(2020, 2, 11, 20, 26, 0, 0, time.UTC))

	return &agentmetricspb.ExportMetricsServiceRequest{
		Node: &occommon.Node{
			Identifier: &occommon.ProcessIdentifier{
				HostName:       "host1",
				Pid:            123,
				StartTimestamp: ts,
			},
			LibraryInfo: &occommon.LibraryInfo{
				Language:           occommon.LibraryInfo_CPP,
				ExporterVersion:    "v1.2.0",
				CoreLibraryVersion: "v2.0.1",
			},
		},
		Resource: &ocresource.Resource{
			Labels: map[string]string{
				"resource-attr": "resource-attr-val-1",
			},
		},
		Metrics: []*ocmetrics.Metric{
			generateOCTestMetricGaugeInt(),
			generateOCTestMetricGaugeDouble(),
			generateOCTestMetricCumulativeInt(),
			generateOCTestMetricCumulativeDouble(),
			generateOCTestMetricDoubleHistogram(),
			generateOCTestMetricDoubleSummary(),
		},
	}
}

func TestMetricsType(t *testing.T) {
	tests := []struct {
		name     string
		internal func() pmetric.Metric
		descType ocmetrics.MetricDescriptor_Type
	}{
		{
			name: "int-gauge",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_INT64,
		},
		{
			name: "double-gauge",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_DOUBLE,
		},
		{
			name: "int-non-monotonic-delta-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(false)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				m.Sum().DataPoints().AppendEmpty().SetIntValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_INT64,
		},
		{
			name: "int-non-monotonic-cumulative-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(false)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				m.Sum().DataPoints().AppendEmpty().SetIntValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_INT64,
		},
		{
			name: "int-monotonic-delta-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(true)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				m.Sum().DataPoints().AppendEmpty().SetIntValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_INT64,
		},
		{
			name: "int-monotonic-cumulative-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(true)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				m.Sum().DataPoints().AppendEmpty().SetIntValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_CUMULATIVE_INT64,
		},
		{
			name: "double-non-monotonic-delta-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(false)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				m.Sum().DataPoints().AppendEmpty().SetDoubleValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_DOUBLE,
		},
		{
			name: "double-non-monotonic-cumulative-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(false)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				m.Sum().DataPoints().AppendEmpty().SetDoubleValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_DOUBLE,
		},
		{
			name: "double-monotonic-delta-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(true)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				m.Sum().DataPoints().AppendEmpty().SetDoubleValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_GAUGE_DOUBLE,
		},
		{
			name: "double-monotonic-cumulative-sum",
			internal: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptySum().SetIsMonotonic(true)
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				m.Sum().DataPoints().AppendEmpty().SetDoubleValue(1)
				return m
			},
			descType: ocmetrics.MetricDescriptor_CUMULATIVE_DOUBLE,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.descType, metricToOC(test.internal()).MetricDescriptor.Type)
		})
	}
}
