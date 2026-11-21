// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNumberTemporality(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func(pmetric.Metric)
		expected string
	}{
		{
			name:     "gauge has no temporality",
			setup:    func(m pmetric.Metric) { m.SetEmptyGauge() },
			expected: "",
		},
		{
			name:     "metric without type has no temporality",
			setup:    func(pmetric.Metric) {},
			expected: "",
		},
		{
			name: "sum with unspecified temporality",
			setup: func(m pmetric.Metric) {
				m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)
			},
			expected: "",
		},
		{
			name: "sum with cumulative temporality",
			setup: func(m pmetric.Metric) {
				m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			},
			expected: "cumulative",
		},
		{
			name: "sum with delta temporality",
			setup: func(m pmetric.Metric) {
				m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
			expected: "delta",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.setup(m)

			dp := pmetric.NewNumberDataPoint()
			dp.SetDoubleValue(30.0)

			assert.Equal(t, tc.expected, NewNumber(m, dp).Temporality())
		})
	}
}
