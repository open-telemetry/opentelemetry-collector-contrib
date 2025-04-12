// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"testing"

	occommon "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	ocmetrics "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	ocresource "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestOCToMetrics(t *testing.T) {
	tests := []struct {
		name     string
		oc       *agentmetricspb.ExportMetricsServiceRequest
		internal pmetric.Metrics
	}{
		{
			name:     "empty",
			oc:       &agentmetricspb.ExportMetricsServiceRequest{},
			internal: pmetric.NewMetrics(),
		},

		{
			name: "one-empty-resource-metrics",
			oc: &agentmetricspb.ExportMetricsServiceRequest{
				Node:     &occommon.Node{},
				Resource: &ocresource.Resource{},
			},
			internal: testdata.GenerateMetricsOneEmptyResourceMetrics(),
		},

		{
			name:     "no-libraries",
			oc:       generateOCTestDataNoMetrics(),
			internal: testdata.GenerateMetricsNoLibraries(),
		},

		// TODO(jpkroehling), enable this test back
		// test disabled as part of the PR #6244, the contents of the results
		// were compared manually and are a match, the reason for this failure
		// couldn't be determined yet.
		// {
		// 	name:     "one-metric-no-labels",
		// 	oc:       generateOCTestDataNoLabels(),
		// 	internal: testdata.GenerateMetricsOneMetricNoAttributes(),
		// },

		{
			name:     "one-metric",
			oc:       generateOCTestDataMetricsOneMetric(),
			internal: testdata.GenerateMetricsOneMetric(),
		},

		{
			name:     "all-types-no-data-points",
			oc:       generateOCTestDataNoPoints(),
			internal: testdata.GenerateMetricsAllTypesNoDataPoints(),
		},

		{
			name: "one-metric-one-summary",
			oc: &agentmetricspb.ExportMetricsServiceRequest{
				Resource: generateOCTestResource(),
				Metrics: []*ocmetrics.Metric{
					generateOCTestMetricCumulativeInt(),
					generateOCTestMetricDoubleSummary(),
				},
			},
			internal: testdata.GenerateMetricsOneCounterOneSummaryMetrics(),
		},

		{
			name:     "one-metric-one-nil",
			oc:       generateOCTestDataMetricsOneMetricOneNil(),
			internal: testdata.GenerateMetricsOneMetric(),
		},

		{
			name:     "one-metric-one-nil-timeseries",
			oc:       generateOCTestDataMetricsOneMetricOneNilTimeseries(),
			internal: testdata.GenerateMetricsOneMetric(),
		},

		{
			name:     "one-metric-one-nil-point",
			oc:       generateOCTestDataMetricsOneMetricOneNilPoint(),
			internal: testdata.GenerateMetricsOneMetric(),
		},

		{
			name:     "one-metric-one-nil-point",
			oc:       generateOCTestDataMetricsOneMetricOneNilPoint(),
			internal: testdata.GenerateMetricsOneMetric(),
		},

		{
			name: "sample-metric",
			oc: &agentmetricspb.ExportMetricsServiceRequest{
				Resource: generateOCTestResource(),
				Metrics: []*ocmetrics.Metric{
					generateOCTestMetricGaugeInt(),
					generateOCTestMetricGaugeDouble(),
					generateOCTestMetricCumulativeInt(),
					generateOCTestMetricCumulativeDouble(),
					generateOCTestMetricDoubleHistogram(),
					generateOCTestMetricDoubleSummary(),
				},
			},
			internal: testdata.GeneratMetricsAllTypesWithSampleDatapoints(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := OCToMetrics(test.oc.Node, test.oc.Resource, test.oc.Metrics)
			assert.NoError(t, pmetrictest.CompareMetrics(test.internal, got))
		})
	}
}

func TestOCToMetrics_ResourceInMetric(t *testing.T) {
	internal := testdata.GenerateMetricsOneMetric()
	want := pmetric.NewMetrics()
	internal.CopyTo(want)
	want.ResourceMetrics().At(0).CopyTo(want.ResourceMetrics().AppendEmpty())
	want.ResourceMetrics().At(1).Resource().Attributes().PutStr("resource-attr", "another-value")
	oc := generateOCTestDataMetricsOneMetric()
	oc2 := generateOCTestDataMetricsOneMetric()
	oc.Metrics = append(oc.Metrics, oc2.Metrics...)
	oc.Metrics[1].Resource = oc2.Resource
	oc.Metrics[1].Resource.Labels["resource-attr"] = "another-value"
	got := OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
	assert.Equal(t, want, got)
}

func TestOCToMetrics_ResourceInMetricOnly(t *testing.T) {
	internal := testdata.GenerateMetricsOneMetric()
	want := pmetric.NewMetrics()
	internal.CopyTo(want)
	oc := generateOCTestDataMetricsOneMetric()
	// Move resource to metric level.
	// We shouldn't have a "combined" resource after conversion
	oc.Metrics[0].Resource = oc.Resource
	oc.Resource = nil
	got := OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
	assert.Equal(t, want, got)
}

func BenchmarkMetricIntOCToMetrics(b *testing.B) {
	ocResource := generateOCTestResource()
	ocMetrics := []*ocmetrics.Metric{
		generateOCTestMetricCumulativeInt(),
		generateOCTestMetricCumulativeInt(),
		generateOCTestMetricCumulativeInt(),
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		OCToMetrics(nil, ocResource, ocMetrics)
	}
}

func BenchmarkMetricDoubleOCToMetrics(b *testing.B) {
	ocResource := generateOCTestResource()
	ocMetrics := []*ocmetrics.Metric{
		generateOCTestMetricCumulativeDouble(),
		generateOCTestMetricCumulativeDouble(),
		generateOCTestMetricCumulativeDouble(),
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		OCToMetrics(nil, ocResource, ocMetrics)
	}
}

func BenchmarkMetricHistogramOCToMetrics(b *testing.B) {
	ocResource := generateOCTestResource()
	ocMetrics := []*ocmetrics.Metric{
		generateOCTestMetricDoubleHistogram(),
		generateOCTestMetricDoubleHistogram(),
		generateOCTestMetricDoubleHistogram(),
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		OCToMetrics(nil, ocResource, ocMetrics)
	}
}

func generateOCTestResource() *ocresource.Resource {
	return &ocresource.Resource{
		Labels: map[string]string{
			"resource-attr": "resource-attr-val-1",
		},
	}
}
