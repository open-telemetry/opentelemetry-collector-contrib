// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestGenDefault(t *testing.T) {
	md := MetricsFromCfg(DefaultCfg())
	require.Equal(t, 1, md.MetricCount())
	require.Equal(t, 1, md.DataPointCount())
	rms := md.ResourceMetrics()
	rm := rms.At(0)
	resource := rm.Resource()
	rattrs := resource.Attributes()
	rattrs.Len()
	require.Equal(t, 1, rattrs.Len())
	val, _ := rattrs.Get("resource-attr-name-0")
	require.Equal(t, "resource-attr-val-0", val.Str())
	ilms := rm.ScopeMetrics()
	require.Equal(t, 1, ilms.Len())
	ms := ilms.At(0).Metrics()
	require.Equal(t, 1, ms.Len())
	pdm := ms.At(0)
	require.Equal(t, "metric_0", pdm.Name())
	require.Equal(t, "my-md-description", pdm.Description())
	require.Equal(t, "my-md-units", pdm.Unit())

	require.Equal(t, pmetric.MetricTypeGauge, pdm.Type())
	pts := pdm.Gauge().DataPoints()
	require.Equal(t, 1, pts.Len())
	pt := pts.At(0)

	require.Equal(t, 1, pt.Attributes().Len())
	ptAttributes, _ := pt.Attributes().Get("pt-label-key-0")
	require.Equal(t, "pt-label-val-0", ptAttributes.Str())

	require.EqualValues(t, 940000000000000000, pt.StartTimestamp())
	require.EqualValues(t, 940000000000000042, pt.Timestamp())
	require.EqualValues(t, 1, pt.IntValue())
}

func TestDoubleHistogramFunctions(t *testing.T) {
	pt := pmetric.NewHistogramDataPoint()
	setDoubleHistogramBounds(pt, 1, 2, 3, 4, 5)
	require.Equal(t, 5, pt.ExplicitBounds().Len())
	require.Equal(t, 5, pt.BucketCounts().Len())

	addDoubleHistogramVal(pt, 1)
	require.EqualValues(t, 1, pt.Count())
	require.EqualValues(t, 1, pt.Sum())
	require.EqualValues(t, 1, pt.BucketCounts().At(0))

	addDoubleHistogramVal(pt, 2)
	require.EqualValues(t, 2, pt.Count())
	require.EqualValues(t, 3, pt.Sum())
	require.EqualValues(t, 1, pt.BucketCounts().At(1))

	addDoubleHistogramVal(pt, 2)
	require.EqualValues(t, 3, pt.Count())
	require.EqualValues(t, 5, pt.Sum())
	require.EqualValues(t, 2, pt.BucketCounts().At(1))
}

func TestGenDoubleHistogram(t *testing.T) {
	cfg := DefaultCfg()
	cfg.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg.PtVal = 2
	md := MetricsFromCfg(cfg)
	pts := getMetric(md).Histogram().DataPoints()
	pt := pts.At(0)
	buckets := pt.BucketCounts()
	require.Equal(t, 5, buckets.Len())
	require.EqualValues(t, 2, buckets.At(2))
}

func TestGenDoubleGauge(t *testing.T) {
	cfg := DefaultCfg()
	cfg.MetricDescriptorType = pmetric.MetricTypeGauge
	md := MetricsFromCfg(cfg)
	metric := getMetric(md)
	pts := metric.Gauge().DataPoints()
	require.Equal(t, 1, pts.Len())
	pt := pts.At(0)
	require.EqualValues(t, float64(1), pt.IntValue())
}

func getMetric(md pmetric.Metrics) pmetric.Metric {
	return md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
}
