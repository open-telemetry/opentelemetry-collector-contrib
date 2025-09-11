// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
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

func TestHandleMetricsPayloadV2(t *testing.T) {
	tests := []struct {
		name                 string
		metricsPayload       gogen.MetricPayload
		expectedSeriesCount  int
		expectedPointsCounts []int
	}{
		{
			name: "v2",
			metricsPayload: gogen.MetricPayload{
				Series: []*gogen.MetricPayload_MetricSeries{
					{
						Resources: []*gogen.MetricPayload_Resource{
							{
								Type: "host",
								Name: "Host1",
							},
						},
						Metric: "system.load.1",
						Tags:   []string{"env:test"},
						Points: []*gogen.MetricPayload_MetricPoint{
							{
								Timestamp: 1636629071,
								Value:     1.5,
							},
							{
								Timestamp: 1636629081,
								Value:     2.0,
							},
						},
						Type: gogen.MetricPayload_COUNT,
					},
				},
			},
			expectedSeriesCount:  1,
			expectedPointsCounts: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := tt.metricsPayload.Marshal()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/api/v2/series", io.NopCloser(bytes.NewReader(pb)))
			require.NoError(t, err)
			mt := createMetricsTranslator()
			series, err := mt.HandleSeriesV2Payload(req)
			require.NoError(t, err)
			require.NoError(t, err, "Failed to parse metrics payload")
			require.Len(t, series, tt.expectedSeriesCount)
			for i, s := range series {
				require.Len(t, s.Points, tt.expectedPointsCounts[i])
			}
		})
	}
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
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestCount", 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 0.5)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 1.0)
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
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireGauge(t, metric, "TestGauge", 2)

				dp := metric.Gauge().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 2)

				dp = metric.Gauge().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 3)
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
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestRate", 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 2)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 3)
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

func TestTranslateSeriesV2(t *testing.T) {
	tests := []struct {
		name   string
		series []*gogen.MetricPayload_MetricSeries
		expect func(t *testing.T, result pmetric.Metrics)
	}{
		{
			name: "Count metric",
			series: []*gogen.MetricPayload_MetricSeries{
				{
					Resources: []*gogen.MetricPayload_Resource{
						{
							Type: "host",
							Name: "Host1",
						},
					},
					Metric: "TestCount",
					Tags:   []string{"env:tag1", "version:tag2"},
					Points: []*gogen.MetricPayload_MetricPoint{
						{
							Timestamp: 1636629071,
							Value:     0.5,
						},
						{
							Timestamp: 1636629081,
							Value:     1.0,
						},
					},
					Type: gogen.MetricPayload_COUNT,
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				expectedAttrs.resource.PutStr("source", "")
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestCount", 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 0.5)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 1.0)
			},
		},
		{
			name: "Gauge metric",
			series: []*gogen.MetricPayload_MetricSeries{
				{
					Resources: []*gogen.MetricPayload_Resource{
						{
							Type: "host",
							Name: "Host1",
						},
					},
					Metric: "TestGauge",
					Tags:   []string{"env:tag1", "version:tag2"},
					Points: []*gogen.MetricPayload_MetricPoint{
						{
							Timestamp: 1636629071,
							Value:     2,
						},
						{
							Timestamp: 1636629081,
							Value:     3,
						},
					},
					Type: gogen.MetricPayload_GAUGE,
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				expectedAttrs.resource.PutStr("source", "")
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireGauge(t, metric, "TestGauge", 2)

				dp := metric.Gauge().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 2)

				dp = metric.Gauge().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 3)
			},
		},
		{
			name: "Rate metric",
			series: []*gogen.MetricPayload_MetricSeries{
				{
					Resources: []*gogen.MetricPayload_Resource{
						{
							Type: "host",
							Name: "Host1",
						},
					},
					Metric: "TestRate",
					Tags:   []string{"env:tag1", "version:tag2"},
					Points: []*gogen.MetricPayload_MetricPoint{
						{
							Timestamp: 1636629071,
							Value:     2,
						},
						{
							Timestamp: 1636629081,
							Value:     3,
						},
					},
					Type: gogen.MetricPayload_RATE,
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				requireMetricAndDataPointCounts(t, result, 1, 2)

				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "Host1", newStringPool())
				expectedAttrs.resource.PutStr("source", "")
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				requireScopeMetrics(t, result, 1, 1)
				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireSum(t, metric, "TestRate", 2)

				dp := metric.Sum().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1636629071, 2)

				dp = metric.Sum().DataPoints().At(1)
				requireDp(t, dp, expectedAttrs.dp, 1636629081, 3)
			},
		},
		{
			name: "Unspecified metric type",
			series: []*gogen.MetricPayload_MetricSeries{
				{
					Resources: []*gogen.MetricPayload_Resource{
						{
							Type: "host",
							Name: "Host1",
						},
					},
					Metric: "TestUnspecified",
					Tags:   []string{"env:tag1", "version:tag2"},
					Points: []*gogen.MetricPayload_MetricPoint{
						{
							Timestamp: 1636629071,
							Value:     2,
						},
						{
							Timestamp: 1636629081,
							Value:     3,
						},
					},
					Type: gogen.MetricPayload_UNSPECIFIED,
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				requireMetricAndDataPointCounts(t, result, 1, 0)

				require.Equal(t, 1, result.ResourceMetrics().Len())
				v, exists := result.ResourceMetrics().At(0).Resource().Attributes().Get(string(semconv.HostNameKey))
				require.True(t, exists)
				require.Equal(t, "Host1", v.AsString())
				v, exists = result.ResourceMetrics().At(0).Resource().Attributes().Get(string(semconv.DeploymentEnvironmentNameKey))
				require.True(t, exists)
				require.Equal(t, "tag1", v.AsString())
				v, exists = result.ResourceMetrics().At(0).Resource().Attributes().Get(string(semconv.ServiceVersionKey))
				require.True(t, exists)
				require.Equal(t, "tag2", v.AsString())

				require.Equal(t, 1, result.ResourceMetrics().At(0).ScopeMetrics().Len())
				require.Equal(t, 1, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

				require.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator", result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
				require.Equal(t, component.NewDefaultBuildInfo().Version, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				require.Equal(t, "TestUnspecified", metric.Name())
				require.Equal(t, pmetric.MetricTypeEmpty, metric.Type())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			result := mt.TranslateSeriesV2(tt.series)

			tt.expect(t, result)
		})
	}
}
