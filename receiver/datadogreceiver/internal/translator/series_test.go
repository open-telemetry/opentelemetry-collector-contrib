// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func strPtr(s string) *string       { return &s }
func float64Ptr(f float64) *float64 { return &f }

type testPoint struct {
	Ts    int64
	Value float64
}

func testPointsToDatadogPoints(points []testPoint) [][]*float64 {
	datadogPoints := make([][]*float64, len(points))
	for i, point := range points {
		datadogPoints[i] = []*float64{float64Ptr(float64(point.Ts)), float64Ptr(point.Value)}
	}
	return datadogPoints

}

func TestTranslateSeriesV1(t *testing.T) {
	tests := []struct {
		name string

		series SeriesList
		expect func(t *testing.T, result pmetric.Metrics)
	}{
		{
			name: "Count metric",
			series: SeriesList{
				Series: []datadogV1.Series{
					{
						Metric: "TestCount",
						Host:   strPtr("Host1"),
						Type:   strPtr(TypeCount),
						Tags:   []string{"env:tag1", "version:tag2"},
						Points: testPointsToDatadogPoints([]testPoint{
							{
								1636629071, 0.5,
							},
							{
								1636629081, 1.0,
							},
						}),
					},
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				expectedResourceAttrs, expectedScopeAttrs, expectedDpAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				requireResourceMetrics(t, result, expectedResourceAttrs, 1)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedScopeAttrs, "otelcol/datadogreceiver", component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestCount", pmetric.AggregationTemporalityDelta, 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedDpAttrs, 1636629071, 0.5)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedDpAttrs, 1636629081, 1.0)
			},
		},
		{
			name: "Gauge metric",
			series: SeriesList{
				Series: []datadogV1.Series{
					{
						Metric: "TestGauge",
						Host:   strPtr("Host1"),
						Type:   strPtr(TypeGauge),
						Tags:   []string{"env:tag1", "version:tag2"},
						Points: testPointsToDatadogPoints([]testPoint{
							{
								Ts:    1636629071,
								Value: 2,
							},
							{
								Ts:    1636629081,
								Value: 3,
							},
						}),
					},
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				expectedResourceAttrs, expectedScopeAttrs, expectedDpAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				requireResourceMetrics(t, result, expectedResourceAttrs, 1)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedScopeAttrs, "otelcol/datadogreceiver", component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireGauge(t, metric, "TestGauge", 2)

				dp := metric.Gauge().DataPoints().At(0)
				requireDp(t, dp, expectedDpAttrs, 1636629071, 2)

				dp = metric.Gauge().DataPoints().At(1)
				requireDp(t, dp, expectedDpAttrs, 1636629081, 3)
			},
		},
		{
			name: "Rate metric",
			series: SeriesList{
				Series: []datadogV1.Series{
					{
						Metric: "TestRate",
						Host:   strPtr("Host1"),
						Type:   strPtr(TypeRate),
						Tags:   []string{"env:tag1", "version:tag2"},
						Points: testPointsToDatadogPoints([]testPoint{
							{
								Ts:    1636629071,
								Value: 2,
							},
							{
								Ts:    1636629081,
								Value: 3,
							},
						}),
					},
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				expectedResourceAttrs, expectedScopeAttrs, expectedDpAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				requireResourceMetrics(t, result, expectedResourceAttrs, 1)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedScopeAttrs, "otelcol/datadogreceiver", component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestRate", pmetric.AggregationTemporalityDelta, 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedDpAttrs, 1636629071, 2)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedDpAttrs, 1636629081, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			result := mt.TranslateSeriesV1(tt.series)

			tt.expect(t, result)
		})
	}
}
