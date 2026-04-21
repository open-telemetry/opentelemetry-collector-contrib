// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstestutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
)

func TestSameMetrics(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	actual := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	diffs := diffMetricData(expected, actual)
	assert.Nil(t, diffs)
}

func TestDifferentValues(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentNumPts(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.NumPtsPerMetric = 2
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestDifferentPtValueTypes(t *testing.T) {
	expected := goldendataset.MetricsFromCfg(goldendataset.DefaultCfg())
	cfg := goldendataset.DefaultCfg()
	cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
	actual := goldendataset.MetricsFromCfg(cfg)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 1)
}

func TestHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeHistogram
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg2.PtVal = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := diffMetricData(expected, actual)
	assert.Len(t, diffs, 3)
}

func TestAttributes(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg1.NumPtLabels = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeHistogram
	cfg2.NumPtLabels = 2
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 1)
}

func TestExponentialHistogram(t *testing.T) {
	cfg1 := goldendataset.DefaultCfg()
	cfg1.MetricDescriptorType = pmetric.MetricTypeExponentialHistogram
	cfg1.PtVal = 1
	expected := goldendataset.MetricsFromCfg(cfg1)
	cfg2 := goldendataset.DefaultCfg()
	cfg2.MetricDescriptorType = pmetric.MetricTypeExponentialHistogram
	cfg2.PtVal = 3
	actual := goldendataset.MetricsFromCfg(cfg2)
	diffs := DiffMetrics(nil, expected, actual)
	assert.Len(t, diffs, 8)
}
