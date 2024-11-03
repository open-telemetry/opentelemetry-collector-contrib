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
		name       string
		moveIf     func(pmetric.ResourceMetrics) bool
		from       pmetric.Metrics
		to         pmetric.Metrics
		expectFrom pmetric.Metrics
		expectTo   pmetric.Metrics
	}{
		{
			name: "move_none",
			moveIf: func(pmetric.ResourceMetrics) bool {
				return false
			},
			from:       pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
			expectTo:   pmetric.NewMetrics(),
		},
		{
			name: "move_all",
			moveIf: func(pmetric.ResourceMetrics) bool {
				return true
			},
			from:       pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetric.NewMetrics(),
			expectTo:   pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
		},
		{
			name: "move_one",
			moveIf: func(rl pmetric.ResourceMetrics) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
			from:       pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
			to:         pmetric.NewMetrics(),
			expectFrom: pmetricutiltest.NewMetrics("B", "CD", "EF", "FG"),
			expectTo:   pmetricutiltest.NewMetrics("A", "CD", "EF", "FG"),
		},
		{
			name: "move_to_preexisting",
			moveIf: func(rl pmetric.ResourceMetrics) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       pmetricutiltest.NewMetrics("AB", "CD", "EF", "FG"),
			to:         pmetricutiltest.NewMetrics("1", "2", "3", "4"),
			expectFrom: pmetricutiltest.NewMetrics("A", "CD", "EF", "FG"),
			expectTo: func() pmetric.Metrics {
				move := pmetricutiltest.NewMetrics("B", "CD", "EF", "FG")
				moveTo := pmetricutiltest.NewMetrics("1", "2", "3", "4")
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
