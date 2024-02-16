// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func initMetric(m pmetric.Metric, name string, ty pmetric.MetricType) {
	m.SetName(name)
	m.SetDescription("")
	m.SetUnit("1")
	switch ty {
	case pmetric.MetricTypeGauge:
		m.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	case pmetric.MetricTypeHistogram:
		histo := m.SetEmptyHistogram()
		histo.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	case pmetric.MetricTypeExponentialHistogram:
		histo := m.SetEmptyExponentialHistogram()
		histo.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
	}
}

func buildHistogramDP(dp pmetric.HistogramDataPoint, timestamp pcommon.Timestamp) {
	dp.SetStartTimestamp(timestamp)
	dp.SetTimestamp(timestamp)
	dp.SetMin(1.0)
	dp.SetMax(2)
	dp.SetCount(5)
	dp.SetSum(7.0)
	dp.BucketCounts().FromRaw([]uint64{3, 2})
	dp.ExplicitBounds().FromRaw([]float64{1, 2})
	dp.Attributes().PutStr("k1", "v1")
}

func buildHistogram(im pmetric.Metric, name string, timestamp pcommon.Timestamp, dpCount int) {
	initMetric(im, name, pmetric.MetricTypeHistogram)
	idps := im.Histogram().DataPoints()
	idps.EnsureCapacity(dpCount)

	for i := 0; i < dpCount; i++ {
		dp := idps.AppendEmpty()
		buildHistogramDP(dp, timestamp)
	}
}

func buildGauge(im pmetric.Metric, name string, timestamp pcommon.Timestamp, dpCount int) {
	initMetric(im, name, pmetric.MetricTypeGauge)
	idps := im.Gauge().DataPoints()
	idps.EnsureCapacity(dpCount)

	for i := 0; i < dpCount; i++ {
		dp := idps.AppendEmpty()
		dp.SetTimestamp(timestamp)
		dp.SetDoubleValue(1000)
		dp.Attributes().PutStr("k1", "v1")
	}
}

func buildSum(im pmetric.Metric, name string, timestamp pcommon.Timestamp, dpCount int) {
	initMetric(im, name, pmetric.MetricTypeSum)
	idps := im.Sum().DataPoints()
	idps.EnsureCapacity(dpCount)

	for i := 0; i < dpCount; i++ {
		dp := idps.AppendEmpty()
		dp.SetStartTimestamp(timestamp)
		dp.SetTimestamp(timestamp)
		dp.SetIntValue(123)
		dp.Attributes().PutStr("k1", "v1")
	}
}

func TestHistogramsAreRetrieved(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Date(2024, 2, 9, 20, 26, 13, 789, time.UTC))
	tests := []struct {
		name            string
		inMetricsFunc   func() pmetric.Metrics
		wantMetricCount int
		wantMetrics     func() pmetric.Metrics
	}{
		{
			name: "no_histograms",
			inMetricsFunc: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(2)
				{
					m := ilm.Metrics().AppendEmpty()
					buildGauge(m, "gauge", ts, 1)
				}
				{
					m := ilm.Metrics().AppendEmpty()
					buildGauge(m, "sum", ts, 1)
				}
				return out
			},
			wantMetricCount: 0,
			wantMetrics:     func() pmetric.Metrics { return pmetric.Metrics{} },
		},
		{
			name: "only_histograms",
			inMetricsFunc: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				ilms := rm.ScopeMetrics()
				ilms.EnsureCapacity(3)
				ilm := ilms.AppendEmpty()
				ilm.SetSchemaUrl("Scope SchemaUrl")
				ilm.Scope().Attributes().PutStr("ks0", "vs0")
				ilm.Scope().SetName("Scope name")
				ilm.Scope().SetVersion("Scope version")
				ilm.Metrics().EnsureCapacity(2)
				{
					m := ilm.Metrics().AppendEmpty()
					buildHistogram(m, "histogram_1", ts, 5)
				}
				{
					m := ilm.Metrics().AppendEmpty()
					buildHistogram(m, "histogram_2", ts, 1)
				}
				return out
			},
			wantMetricCount: 2,
			wantMetrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				ilm := rm.ScopeMetrics().AppendEmpty()
				ilm.SetSchemaUrl("Scope SchemaUrl")
				ilm.Scope().Attributes().PutStr("ks0", "vs0")
				ilm.Scope().SetName("Scope name")
				ilm.Scope().SetVersion("Scope version")
				ilm.Metrics().EnsureCapacity(2)
				{
					m := ilm.Metrics().AppendEmpty()
					buildHistogram(m, "histogram_1", ts, 5)
				}
				{
					m := ilm.Metrics().AppendEmpty()
					buildHistogram(m, "histogram_2", ts, 1)
				}
				return out
			},
		},
		{
			name: "mixed_type_multiple_scopes",
			inMetricsFunc: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				rm.ScopeMetrics().AppendEmpty()
				ilm0 := rm.ScopeMetrics().At(0)
				ilm0.SetSchemaUrl("Scope SchemaUrl")
				ilm0.Scope().Attributes().PutStr("ks0", "vs0")
				ilm0.Scope().SetName("Scope name")
				ilm0.Scope().SetVersion("Scope version")
				ilm0.Metrics().EnsureCapacity(2)
				ilm0.Metrics().AppendEmpty()
				buildHistogram(ilm0.Metrics().At(0), "histogram_1", ts, 1)
				ilm0.Metrics().AppendEmpty()
				buildGauge(ilm0.Metrics().At(1), "gauge", ts, 2)

				rm.ScopeMetrics().AppendEmpty()
				ilm1 := rm.ScopeMetrics().At(1)
				ilm1.Metrics().AppendEmpty()
				buildSum(ilm1.Metrics().At(0), "gauge", ts, 2)

				rm.ScopeMetrics().AppendEmpty()
				ilm2 := rm.ScopeMetrics().At(2)
				ilm2.SetSchemaUrl("Scope SchemaUrl")
				ilm2.Scope().Attributes().PutStr("ks0", "vs0")
				ilm2.Metrics().EnsureCapacity(2)
				ilm2.Metrics().AppendEmpty()
				buildHistogram(ilm2.Metrics().At(0), "histogram_1", ts, 1)
				ilm2.Metrics().AppendEmpty()
				buildHistogram(ilm2.Metrics().At(1), "histogram_2", ts, 2)
				return out
			},
			wantMetricCount: 3,
			wantMetrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				rm.ScopeMetrics().AppendEmpty()
				ilm0 := rm.ScopeMetrics().At(0)
				ilm0.SetSchemaUrl("Scope SchemaUrl")
				ilm0.Scope().Attributes().PutStr("ks0", "vs0")
				ilm0.Scope().SetName("Scope name")
				ilm0.Scope().SetVersion("Scope version")
				buildHistogram(ilm0.Metrics().AppendEmpty(), "histogram_1", ts, 1)

				rm.ScopeMetrics().AppendEmpty()
				ilm1 := rm.ScopeMetrics().At(1)
				ilm1.SetSchemaUrl("Scope SchemaUrl")
				ilm1.Scope().Attributes().PutStr("ks0", "vs0")
				ilm1.Metrics().EnsureCapacity(2)
				ilm1.Metrics().AppendEmpty()
				buildHistogram(ilm1.Metrics().At(0), "histogram_1", ts, 1)
				ilm1.Metrics().AppendEmpty()
				buildHistogram(ilm1.Metrics().At(1), "histogram_2", ts, 2)
				return out
			}},
		{
			name: "mixed_type_multiple_resources",
			inMetricsFunc: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				out.ResourceMetrics().EnsureCapacity(2)
				out.ResourceMetrics().AppendEmpty()
				rm0 := out.ResourceMetrics().At(0)
				rm0.SetSchemaUrl("Resource SchemaUrl")
				rm0.Resource().Attributes().PutStr("kr0", "vr0")
				rm0.ScopeMetrics().AppendEmpty()
				ilm0r0 := rm0.ScopeMetrics().At(0)
				ilm0r0.SetSchemaUrl("Scope SchemaUrl")
				ilm0r0.Scope().Attributes().PutStr("ks0", "vs0")
				ilm0r0.Metrics().EnsureCapacity(2)
				ilm0r0.Metrics().AppendEmpty()
				buildHistogram(ilm0r0.Metrics().At(0), "histogram_1", ts, 1)
				ilm0r0.Metrics().AppendEmpty()
				buildGauge(ilm0r0.Metrics().At(1), "gauge", ts, 1)
				rm0.ScopeMetrics().AppendEmpty()
				ilm1r0 := rm0.ScopeMetrics().At(1)
				ilm1r0.Metrics().AppendEmpty()
				buildGauge(ilm1r0.Metrics().At(0), "gauge", ts, 1)

				out.ResourceMetrics().AppendEmpty()
				rm1 := out.ResourceMetrics().At(1)
				rm1.Resource().Attributes().PutStr("kr1", "vr1")
				ilm0r1 := rm1.ScopeMetrics().AppendEmpty()
				ilm0r1.SetSchemaUrl("Scope SchemaUrl")
				ilm0r1.Scope().Attributes().PutStr("ks0", "vs0")
				ilm0r1.Metrics().AppendEmpty()
				buildGauge(ilm0r1.Metrics().At(0), "gauge", ts, 1)

				return out
			},
			wantMetricCount: 1,
			wantMetrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				out.ResourceMetrics().AppendEmpty()
				rm := out.ResourceMetrics().At(0)
				rm.SetSchemaUrl("Resource SchemaUrl")
				rm.Resource().Attributes().PutStr("kr0", "vr0")
				rm.ScopeMetrics().AppendEmpty()
				ilm0 := rm.ScopeMetrics().At(0)
				ilm0.SetSchemaUrl("Scope SchemaUrl")
				ilm0.Scope().Attributes().PutStr("ks0", "vs0")
				ilm0.Metrics().EnsureCapacity(1)
				ilm0.Metrics().AppendEmpty()
				buildHistogram(ilm0.Metrics().At(0), "histogram_1", ts, 1)
				return out
			}},
		{
			name: "remove_access_token",
			inMetricsFunc: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				res.Attributes().PutStr("com.splunk.signalfx.access_token", "abcd")
				ilms := rm.ScopeMetrics()
				ilms.EnsureCapacity(3)
				ilm := ilms.AppendEmpty()
				buildHistogram(ilm.Metrics().AppendEmpty(), "histogram_1", ts, 1)
				return out
			},
			wantMetricCount: 1,
			wantMetrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("kr0", "vr0")
				ilms := rm.ScopeMetrics()
				ilms.EnsureCapacity(3)
				ilm := ilms.AppendEmpty()
				buildHistogram(ilm.Metrics().AppendEmpty(), "histogram_1", ts, 1)
				return out
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := tt.inMetricsFunc()
			gotMetrics, gotCount := GetHistograms(md)

			assert.Equal(t, tt.wantMetricCount, gotCount)
			assert.Equal(t, tt.wantMetrics(), gotMetrics)
		})
	}
}
