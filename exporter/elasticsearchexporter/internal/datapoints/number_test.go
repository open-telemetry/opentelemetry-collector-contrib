// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNumberTemporality(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("test")

	dp := pmetric.NewNumberDataPoint()
	dp.SetDoubleValue(30.0)

	esSum := NewNumber(m, dp)

	actual := esSum.Temporality()
	assert.Empty(t, actual, "number doesn't have a temporality")
}

func TestNumberExemplars(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dp       pmetric.NumberDataPoint
		expected []any
	}{
		{
			name: "returns double",
			dp: func() pmetric.NumberDataPoint {
				dp := pmetric.NewNumberDataPoint()
				dp.SetDoubleValue(10)
				dp.Exemplars().AppendEmpty().SetDoubleValue(10)
				dp.Exemplars().AppendEmpty().SetDoubleValue(20)
				return dp
			}(),
			expected: []any{10.0, 20.0},
		},
		{
			name: "returns int",
			dp: func() pmetric.NumberDataPoint {
				dp := pmetric.NewNumberDataPoint()
				dp.SetDoubleValue(10)
				dp.Exemplars().AppendEmpty().SetIntValue(10)
				dp.Exemplars().AppendEmpty().SetIntValue(20)
				return dp
			}(),
			expected: []any{int64(10), int64(20)},
		},
		{
			name: "returns trace and span ID",
			dp: func() pmetric.NumberDataPoint {
				dp := pmetric.NewNumberDataPoint()

				e := dp.Exemplars().AppendEmpty()
				e.SetDoubleValue(10)
				e.SetTraceID(pcommon.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10})
				e.SetSpanID(pcommon.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
				e.FilteredAttributes().PutInt("thread.id", 42)
				return dp
			}(),
			expected: []any{
				pcommon.TraceID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				pcommon.SpanID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
				10.0,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")

			esHist := NewNumber(m, tc.dp)
			actual := esHist.Exemplars()
			assert.Equal(t, tc.expected, exemplarValues(t, actual))
		})
	}
}
