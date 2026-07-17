// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestOfMetricCapturesMetricSpecificFields(t *testing.T) {
	t.Parallel()

	scopeID := newTestScopeIdentity(t)

	tests := []struct {
		name            string
		buildMetric     func() pmetric.Metric
		wantType        pmetric.MetricType
		wantMonotonic   bool
		wantTemporality pmetric.AggregationTemporality
	}{
		{
			name: "gauge",
			buildMetric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("queue.depth")
				metric.SetUnit("1")
				metric.SetEmptyGauge()
				return metric
			},
			wantType: pmetric.MetricTypeGauge,
		},
		{
			name: "sum",
			buildMetric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("requests")
				metric.SetUnit("1")
				sum := metric.SetEmptySum()
				sum.SetIsMonotonic(false)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return metric
			},
			wantType:        pmetric.MetricTypeSum,
			wantTemporality: pmetric.AggregationTemporalityCumulative,
		},
		{
			name: "histogram",
			buildMetric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("request.duration")
				metric.SetUnit("ms")
				histogram := metric.SetEmptyHistogram()
				histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				return metric
			},
			wantType:        pmetric.MetricTypeHistogram,
			wantMonotonic:   true,
			wantTemporality: pmetric.AggregationTemporalityDelta,
		},
		{
			name: "exponential histogram",
			buildMetric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("payload.size")
				metric.SetUnit("By")
				histogram := metric.SetEmptyExponentialHistogram()
				histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return metric
			},
			wantType:        pmetric.MetricTypeExponentialHistogram,
			wantMonotonic:   true,
			wantTemporality: pmetric.AggregationTemporalityCumulative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metric := tt.buildMetric()
			got := OfMetric(scopeID, metric)

			require.Equal(t, scopeID, got.Scope())
			require.Equal(t, metric.Name(), got.name)
			require.Equal(t, metric.Unit(), got.unit)
			require.Equal(t, tt.wantType, got.ty)
			require.Equal(t, tt.wantMonotonic, got.monotonic)
			require.Equal(t, tt.wantTemporality, got.temporality)
		})
	}
}

func TestMetricHashChangesWhenIdentityChanges(t *testing.T) {
	t.Parallel()

	resource, scope := newTestResourceAndScope(t)
	base := OfResourceMetric(resource, scope, newSumMetric("requests", "1", true, pmetric.AggregationTemporalityDelta))
	baseHash := base.Hash().Sum64()

	_, scopeChanged := newTestResourceAndScope(t)
	scopeChanged.SetVersion("v9.9.9")
	require.NotEqual(t, baseHash, OfResourceMetric(resource, scopeChanged, newSumMetric("requests", "1", true, pmetric.AggregationTemporalityDelta)).Hash().Sum64())

	require.NotEqual(t, baseHash, OfResourceMetric(resource, scope, newSumMetric("errors", "1", true, pmetric.AggregationTemporalityDelta)).Hash().Sum64())
	require.NotEqual(t, baseHash, OfResourceMetric(resource, scope, newSumMetric("requests", "ms", true, pmetric.AggregationTemporalityDelta)).Hash().Sum64())
	require.NotEqual(t, baseHash, OfResourceMetric(resource, scope, newSumMetric("requests", "1", false, pmetric.AggregationTemporalityDelta)).Hash().Sum64())
	require.NotEqual(t, baseHash, OfResourceMetric(resource, scope, newSumMetric("requests", "1", true, pmetric.AggregationTemporalityCumulative)).Hash().Sum64())
}

func TestOfResourceMetricMatchesNestedConstructors(t *testing.T) {
	t.Parallel()

	resource, scope := newTestResourceAndScope(t)
	metric := newSumMetric("requests", "1", true, pmetric.AggregationTemporalityDelta)

	require.Equal(
		t,
		OfMetric(OfScope(OfResource(resource), scope), metric),
		OfResourceMetric(resource, scope, metric),
	)
}

func TestMetricStringUsesMetricPrefixAndHash(t *testing.T) {
	t.Parallel()

	scopeID := newTestScopeIdentity(t)
	metricID := OfMetric(scopeID, newSumMetric("requests", "1", true, pmetric.AggregationTemporalityDelta))

	require.Equal(t, fmt.Sprintf("metric/%x", metricID.Hash().Sum64()), metricID.String())
}

func newTestScopeIdentity(t *testing.T) Scope {
	t.Helper()

	resource, scope := newTestResourceAndScope(t)
	return OfScope(OfResource(resource), scope)
}

func newTestResourceAndScope(t *testing.T) (pcommon.Resource, pcommon.InstrumentationScope) {
	t.Helper()

	resource := pcommon.NewResource()
	err := resource.Attributes().FromRaw(map[string]any{
		"service.name": "checkout",
		"service.env":  "prod",
	})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("receiver")
	scope.SetVersion("v1.2.3")
	err = scope.Attributes().FromRaw(map[string]any{
		"library": "otlp",
	})
	require.NoError(t, err)

	return resource, scope
}

func newSumMetric(name, unit string, monotonic bool, temporality pmetric.AggregationTemporality) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetUnit(unit)

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(monotonic)
	sum.SetAggregationTemporality(temporality)

	return metric
}
