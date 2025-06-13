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
			checkAssertions(t, want, actual)
		}
	}
}

func checkAssertions(t *testing.T, want pmetric.Metric, actual pmetric.Metric) {
	var wantDps, actualDps pmetric.NumberDataPointSlice

	// Get datapoints based on metric type
	if want.Type() == pmetric.MetricTypeGauge {
		wantDps = want.Gauge().DataPoints()
		actualDps = actual.Gauge().DataPoints()
	} else {
		wantDps = want.Sum().DataPoints()
		actualDps = actual.Sum().DataPoints()
	}

	assert.Equal(t, wantDps.Len(), actualDps.Len(), "Datapoint count mismatch")

	// Iterate through all datapoints
	for i := 0; i < wantDps.Len(); i++ {
		wantDp := wantDps.At(i)
		actualDp := actualDps.At(i)
		assert.Equal(t, wantDp.DoubleValue(), actualDp.DoubleValue(), "Datapoint value mismatch")
		wantAttrs := wantDp.Attributes()
		actualAttrs := actualDp.Attributes()
		assert.Equal(t, wantAttrs.Len(), actualAttrs.Len(), "Attribute count mismatch")
		wantAttrs.Range(func(k string, v pcommon.Value) bool {
			av, ok := actualAttrs.Get(k)
			assert.True(t, ok, "Missing attribute: %s", k)
			assert.Equal(t, v, av, "Attribute value mismatch for %s", k)
			return true
		})
	}
}

func GenerateMetrics(nameToDimsGauges map[MetricIdentifier][]map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	for metric, dims := range nameToDimsGauges {
		m := ms.AppendEmpty()
		var dps pmetric.NumberDataPointSlice
		if metric.MetricType == pmetric.MetricTypeSum {
			m.SetEmptySum()
			dps = m.Sum().DataPoints()
		} else {
			m.SetEmptyGauge()
			dps = m.Gauge().DataPoints()
		}
		m.SetName(metric.Name)
		for _, dim := range dims {
			metricBody := dps.AppendEmpty()
			metricBody.SetDoubleValue(metric.DataValue)
			for k, v := range dim {
				if k == "Unit" {
					m.SetUnit(v)
				} else {
					metricBody.Attributes().PutStr(k, v)
				}
			}
		}
	}
	return md
}
