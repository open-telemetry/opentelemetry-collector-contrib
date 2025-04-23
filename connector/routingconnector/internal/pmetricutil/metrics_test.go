// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/pmetricutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestMoveResourcesIf(t *testing.T) {
	testCases := []struct {
		from       pmetric.Metrics
		to         pmetric.Metrics
		expectFrom pmetric.Metrics
		expectTo   pmetric.Metrics
		moveIf     func(pmetric.ResourceMetrics) bool
		name       string
	}{
		{
			name: "move_none",
			moveIf: func(pmetric.ResourceMetrics) bool {
				return false
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "move_all",
			moveIf: func(pmetric.ResourceMetrics) bool {
				return true
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
		},
		{
			name: "move_one",
			moveIf: func(rl pmetric.ResourceMetrics) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("B", "CD", "EF", "FG"),
			expectTo:   pmetricutiltest.NewGauges("A", "CD", "EF", "FG"),
		},
		{
			name: "move_to_preexisting",
			moveIf: func(rl pmetric.ResourceMetrics) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "FG"),
			to:         pmetricutiltest.NewGauges("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewGauges("A", "CD", "EF", "FG"),
			expectTo: func() pmetric.Metrics {
				move := pmetricutiltest.NewGauges("B", "CD", "EF", "FG")
				moveTo := pmetricutiltest.NewGauges("1", "2", "3", "4")
				move.ResourceMetrics().MoveAndAppendTo(moveTo.ResourceMetrics())
				return moveTo
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			pmetricutil.MoveResourcesIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func TestMoveMetricsWithContextIf(t *testing.T) {
	testCases := []struct {
		from       pmetric.Metrics
		to         pmetric.Metrics
		expectFrom pmetric.Metrics
		expectTo   pmetric.Metrics
		moveIf     func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric) bool
		name       string
	}{
		{
			name: "move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				return false
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				return true
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
		},
		{
			name: "move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("B", "C", "EF", "GH"),
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewGauges("AB", "D", "EF", "GH"),
		},
		{
			name: "move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("A", "D", "F", "GH"),
		},
		{
			name: "move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
		},
		{
			name: "move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("B", "CD", "E", "GH"),
		},
		{
			name: "move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewGauges("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewGauges("AB", "C", "EF", "GH"),
			expectTo: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("1", pmetricutiltest.Scope("2",
					pmetricutiltest.Gauge("3", pmetricutiltest.NumberDataPoint("4")),
				)),
				pmetricutiltest.Resource("A", pmetricutiltest.Scope("D",
					pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
				)),
				pmetricutiltest.Resource("B", pmetricutiltest.Scope("D",
					pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
				)),
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			pmetricutil.MoveMetricsWithContextIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func TestMoveDataPointsWithContextIf(t *testing.T) {
	testCases := []struct {
		from       pmetric.Metrics
		to         pmetric.Metrics
		expectFrom pmetric.Metrics
		expectTo   pmetric.Metrics
		moveIf     func(pmetric.ResourceMetrics, pmetric.ScopeMetrics, pmetric.Metric, any) bool
		name       string
	}{
		// gauge
		{
			name: "gauge/move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return false
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "gauge/move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return true
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
		},
		{
			name: "gauge/move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewGauges("B", "CD", "EF", "GH"),
		},
		{
			name: "gauge/move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("B", "C", "EF", "GH"),
		},
		{
			name: "gauge/move_all_from_one_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
		},
		{
			name: "gauge/move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewGauges("AB", "D", "EF", "GH"),
		},
		{
			name: "gauge/move_all_from_one_metric_in_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricF"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "E", "GH"),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "F", "GH"),
		},
		{
			name: "gauge/move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				rname, ok1 := rl.Resource().Attributes().Get("resourceName")
				dpname, ok2 := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok1 && ok2 && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("A", "D", "F", "G"),
		},
		{
			name: "gauge/move_one_from_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && sl.Scope().Name() == "scopeD" && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("AB", "D", "E", "G"),
		},
		{
			name: "gauge/move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("AB", "CD", "E", "G"),
		},
		{
			name: "gauge/move_one_from_each_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectTo:   pmetricutiltest.NewGauges("AB", "CD", "EF", "G"),
		},
		{
			name: "gauge/move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Gauge("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewGauges("B", "CD", "E", "GH"),
		},
		{
			name: "gauge/move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewGauges("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewGauges("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewGauges("AB", "CD", "EF", "H"),
			expectTo: func() pmetric.Metrics {
				orig := pmetricutiltest.NewGauges("1", "2", "3", "4")
				extra := pmetricutiltest.NewGauges("AB", "CD", "EF", "G")
				extra.ResourceMetrics().MoveAndAppendTo(orig.ResourceMetrics())
				return orig
			}(),
		},

		// sum
		{
			name: "sum/move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return false
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "sum/move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return true
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
		},
		{
			name: "sum/move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewSums("B", "CD", "EF", "GH"),
		},
		{
			name: "sum/move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSums("B", "C", "EF", "GH"),
		},
		{
			name: "sum/move_all_from_one_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewSums("AB", "CD", "E", "GH"),
		},
		{
			name: "sum/move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewSums("AB", "D", "EF", "GH"),
		},
		{
			name: "sum/move_all_from_one_metric_in_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricF"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("AB", "CD", "E", "GH"),
			expectTo:   pmetricutiltest.NewSums("AB", "CD", "F", "GH"),
		},
		{
			name: "sum/move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				rname, ok1 := rl.Resource().Attributes().Get("resourceName")
				dpname, ok2 := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok1 && ok2 && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSums("A", "D", "F", "G"),
		},
		{
			name: "sum/move_one_from_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && sl.Scope().Name() == "scopeD" && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSums("AB", "D", "E", "G"),
		},
		{
			name: "sum/move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSums("AB", "CD", "E", "G"),
		},
		{
			name: "sum/move_one_from_each_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSums("AB", "CD", "EF", "H"),
			expectTo:   pmetricutiltest.NewSums("AB", "CD", "EF", "G"),
		},
		{
			name: "sum/move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("E", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Sum("F", pmetricutiltest.NumberDataPoint("G"), pmetricutiltest.NumberDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSums("B", "CD", "E", "GH"),
		},
		{
			name: "sum/move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.NumberDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewSums("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewSums("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewSums("AB", "CD", "EF", "H"),
			expectTo: func() pmetric.Metrics {
				orig := pmetricutiltest.NewSums("1", "2", "3", "4")
				extra := pmetricutiltest.NewSums("AB", "CD", "EF", "G")
				extra.ResourceMetrics().MoveAndAppendTo(orig.ResourceMetrics())
				return orig
			}(),
		},

		// histogram
		{
			name: "histogram/move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return false
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "histogram/move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return true
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
		},
		{
			name: "histogram/move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewHistograms("B", "CD", "EF", "GH"),
		},
		{
			name: "histogram/move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewHistograms("B", "C", "EF", "GH"),
		},
		{
			name: "histogram/move_all_from_one_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewHistograms("AB", "CD", "E", "GH"),
		},
		{
			name: "histogram/move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewHistograms("AB", "D", "EF", "GH"),
		},
		{
			name: "histogram/move_all_from_one_metric_in_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricF"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("AB", "CD", "E", "GH"),
			expectTo:   pmetricutiltest.NewHistograms("AB", "CD", "F", "GH"),
		},
		{
			name: "histogram/move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				rname, ok1 := rl.Resource().Attributes().Get("resourceName")
				dpname, ok2 := dp.(pmetric.HistogramDataPoint).Attributes().Get("dpName")
				return ok1 && ok2 && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewHistograms("A", "D", "F", "G"),
		},
		{
			name: "histogram/move_one_from_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.HistogramDataPoint).Attributes().Get("dpName")
				return ok && sl.Scope().Name() == "scopeD" && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewHistograms("AB", "D", "E", "G"),
		},
		{
			name: "histogram/move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.HistogramDataPoint).Attributes().Get("dpName")
				return ok && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewHistograms("AB", "CD", "E", "G"),
		},
		{
			name: "histogram/move_one_from_each_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.HistogramDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewHistograms("AB", "CD", "EF", "H"),
			expectTo:   pmetricutiltest.NewHistograms("AB", "CD", "EF", "G"),
		},
		{
			name: "histogram/move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("E", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Histogram("F", pmetricutiltest.HistogramDataPoint("G"), pmetricutiltest.HistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewHistograms("B", "CD", "E", "GH"),
		},
		{
			name: "histogram/move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.HistogramDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewHistograms("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewHistograms("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewHistograms("AB", "CD", "EF", "H"),
			expectTo: func() pmetric.Metrics {
				orig := pmetricutiltest.NewHistograms("1", "2", "3", "4")
				extra := pmetricutiltest.NewHistograms("AB", "CD", "EF", "G")
				extra.ResourceMetrics().MoveAndAppendTo(orig.ResourceMetrics())
				return orig
			}(),
		},

		// exponential_histogram
		{
			name: "exponential_histogram/move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return false
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "exponential_histogram/move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return true
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
		},
		{
			name: "exponential_histogram/move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewExponentialHistograms("B", "CD", "EF", "GH"),
		},
		{
			name: "exponential_histogram/move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewExponentialHistograms("B", "C", "EF", "GH"),
		},
		{
			name: "exponential_histogram/move_all_from_one_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewExponentialHistograms("AB", "CD", "E", "GH"),
		},
		{
			name: "exponential_histogram/move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewExponentialHistograms("AB", "D", "EF", "GH"),
		},
		{
			name: "exponential_histogram/move_all_from_one_metric_in_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricF"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "CD", "E", "GH"),
			expectTo:   pmetricutiltest.NewExponentialHistograms("AB", "CD", "F", "GH"),
		},
		{
			name: "exponential_histogram/move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				rname, ok1 := rl.Resource().Attributes().Get("resourceName")
				dpname, ok2 := dp.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("dpName")
				return ok1 && ok2 && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewExponentialHistograms("A", "D", "F", "G"),
		},
		{
			name: "exponential_histogram/move_one_from_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("dpName")
				return ok && sl.Scope().Name() == "scopeD" && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewExponentialHistograms("AB", "D", "E", "G"),
		},
		{
			name: "exponential_histogram/move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("dpName")
				return ok && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewExponentialHistograms("AB", "CD", "E", "G"),
		},
		{
			name: "exponential_histogram/move_one_from_each_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "H"),
			expectTo:   pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "G"),
		},
		{
			name: "exponential_histogram/move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("E", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.ExponentialHistogram("F", pmetricutiltest.ExponentialHistogramDataPoint("G"), pmetricutiltest.ExponentialHistogramDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewExponentialHistograms("B", "CD", "E", "GH"),
		},
		{
			name: "exponential_histogram/move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.ExponentialHistogramDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewExponentialHistograms("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "H"),
			expectTo: func() pmetric.Metrics {
				orig := pmetricutiltest.NewExponentialHistograms("1", "2", "3", "4")
				extra := pmetricutiltest.NewExponentialHistograms("AB", "CD", "EF", "G")
				extra.ResourceMetrics().MoveAndAppendTo(orig.ResourceMetrics())
				return orig
			}(),
		},

		// summary
		{
			name: "summary/move_none",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return false
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "summary/move_all",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return true
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
		},
		{
			name: "summary/move_all_from_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("A", "CD", "EF", "GH"),
			expectTo:   pmetricutiltest.NewSummaries("B", "CD", "EF", "GH"),
		},
		{
			name: "summary/move_all_from_one_scope",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSummaries("B", "C", "EF", "GH"),
		},
		{
			name: "summary/move_all_from_one_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricE"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("AB", "CD", "F", "GH"),
			expectTo:   pmetricutiltest.NewSummaries("AB", "CD", "E", "GH"),
		},
		{
			name: "summary/move_all_from_one_scope_in_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, _ pmetric.Metric, _ any) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("AB", "C", "EF", "GH"),
			expectTo:   pmetricutiltest.NewSummaries("AB", "D", "EF", "GH"),
		},
		{
			name: "summary/move_all_from_one_metric_in_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				return m.Name() == "metricF"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("AB", "CD", "E", "GH"),
			expectTo:   pmetricutiltest.NewSummaries("AB", "CD", "F", "GH"),
		},
		{
			name: "summary/move_one",
			moveIf: func(rl pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				rname, ok1 := rl.Resource().Attributes().Get("resourceName")
				dpname, ok2 := dp.(pmetric.SummaryDataPoint).Attributes().Get("dpName")
				return ok1 && ok2 && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && m.Name() == "metricF" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSummaries("A", "D", "F", "G"),
		},
		{
			name: "summary/move_one_from_each_resource",
			moveIf: func(_ pmetric.ResourceMetrics, sl pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.SummaryDataPoint).Attributes().Get("dpName")
				return ok && sl.Scope().Name() == "scopeD" && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSummaries("AB", "D", "E", "G"),
		},
		{
			name: "summary/move_one_from_each_scope",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.SummaryDataPoint).Attributes().Get("dpName")
				return ok && m.Name() == "metricE" && dpname.AsString() == "dpG"
			},
			from: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSummaries("AB", "CD", "E", "G"),
		},
		{
			name: "summary/move_one_from_each_metric",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.SummaryDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewSummaries("AB", "CD", "EF", "H"),
			expectTo:   pmetricutiltest.NewSummaries("AB", "CD", "EF", "G"),
		},
		{
			name: "summary/move_one_from_each_scope_in_one_resource",
			moveIf: func(rl pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, m pmetric.Metric, _ any) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && m.Name() == "metricE"
			},
			from: pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:   pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetricsFromOpts(
				pmetricutiltest.Resource("A",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("E", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
				pmetricutiltest.Resource("B",
					pmetricutiltest.Scope("C",
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
					pmetricutiltest.Scope("D",
						pmetricutiltest.Summary("F", pmetricutiltest.SummaryDataPoint("G"), pmetricutiltest.SummaryDataPoint("H")),
					),
				),
			),
			expectTo: pmetricutiltest.NewSummaries("B", "CD", "E", "GH"),
		},
		{
			name: "summary/move_some_to_preexisting",
			moveIf: func(_ pmetric.ResourceMetrics, _ pmetric.ScopeMetrics, _ pmetric.Metric, dp any) bool {
				dpname, ok := dp.(pmetric.SummaryDataPoint).Attributes().Get("dpName")
				return ok && dpname.AsString() == "dpG"
			},
			from:       pmetricutiltest.NewSummaries("AB", "CD", "EF", "GH"),
			to:         pmetricutiltest.NewSummaries("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewSummaries("AB", "CD", "EF", "H"),
			expectTo: func() pmetric.Metrics {
				orig := pmetricutiltest.NewSummaries("1", "2", "3", "4")
				extra := pmetricutiltest.NewSummaries("AB", "CD", "EF", "G")
				extra.ResourceMetrics().MoveAndAppendTo(orig.ResourceMetrics())
				return orig
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			pmetricutil.MoveDataPointsWithContextIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func BenchmarkMoveResourcesIfMetrics(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		from := pmetricutiltest.NewGauges("AB", "CD", "EF", "GH")
		to := pmetric.NewMetrics()
		pmetricutil.MoveResourcesIf(from, to, func(pmetric.ResourceMetrics) bool {
			return true
		})
		assert.Equal(b, 0, from.DataPointCount())
		assert.Equal(b, 16, to.DataPointCount())
	}
}
