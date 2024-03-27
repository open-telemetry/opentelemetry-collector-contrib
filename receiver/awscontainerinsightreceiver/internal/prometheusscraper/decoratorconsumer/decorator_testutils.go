// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package decoratorconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper/decoratorconsumer"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type MetricIdentifier struct {
	Name       string
	MetricType pmetric.MetricType
	DataValue  float64
}

type TestCase struct {
	Metrics     pmetric.Metrics
	Want        pmetric.Metrics
	ShouldError bool
}

func RunDecoratorTestScenarios(ctx context.Context, t *testing.T, dc consumer.Metrics, testcases map[string]TestCase) {
	for _, tc := range testcases {
		err := dc.ConsumeMetrics(ctx, tc.Metrics)
		if tc.ShouldError {
			assert.Error(t, err)
			return
		}
		require.NoError(t, err)
		assert.Equal(t, tc.Want.MetricCount(), tc.Metrics.MetricCount())
		if tc.Want.MetricCount() == 0 {
			continue
		}
		actuals := tc.Metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		actuals.Sort(func(a, b pmetric.Metric) bool {
			return a.Name() < b.Name()
		})
		wants := tc.Want.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		wants.Sort(func(a, b pmetric.Metric) bool {
			return a.Name() < b.Name()
		})
		for i := 0; i < wants.Len(); i++ {
			actual := actuals.At(i)
			want := wants.At(i)
			assert.Equal(t, want.Name(), actual.Name())
			assert.Equal(t, want.Unit(), actual.Unit())
			assert.Equal(t, getDataValue(&want), getDataValue(&actual))
			actualAttrs := getAttributesFromMetric(&actual)
			wantAttrs := getAttributesFromMetric(&want)
			assert.Equal(t, wantAttrs.Len(), actualAttrs.Len())
			wantAttrs.Range(func(k string, v pcommon.Value) bool {
				av, ok := actualAttrs.Get(k)
				assert.True(t, ok)
				assert.Equal(t, v, av)
				return true
			})
		}
	}
}

func GenerateMetrics(nameToDimsGauges map[MetricIdentifier][]map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	for metric, dims := range nameToDimsGauges {
		for _, dim := range dims {
			m := ms.AppendEmpty()
			m.SetName(metric.Name)
			metricBody := m.SetEmptyGauge().DataPoints().AppendEmpty()
			if metric.MetricType == pmetric.MetricTypeSum {
				metricBody = m.SetEmptySum().DataPoints().AppendEmpty()
			}
			metricBody.SetDoubleValue(metric.DataValue)
			for k, v := range dim {
				if k == "Unit" {
					m.SetUnit(v)
					continue
				}
				metricBody.Attributes().PutStr(k, v)
			}
		}
	}
	return md
}

func getAttributesFromMetric(m *pmetric.Metric) pcommon.Map {
	if m.Type() == pmetric.MetricTypeGauge {
		return m.Gauge().DataPoints().At(0).Attributes()
	}
	return m.Sum().DataPoints().At(0).Attributes()
}

func getDataValue(m *pmetric.Metric) float64 {
	if m.Type() == pmetric.MetricTypeGauge {
		return m.Gauge().DataPoints().At(0).DoubleValue()
	}
	return m.Sum().DataPoints().At(0).DoubleValue()
}
