// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func exemplarValues(t *testing.T, s pmetric.ExemplarSlice) []any {
	t.Helper()
	out := make([]any, 0, s.Len())
	for i := range s.Len() {
		e := s.At(i)

		if !e.TraceID().IsEmpty() {
			out = append(out, e.TraceID())
		}
		if !e.SpanID().IsEmpty() {
			out = append(out, e.SpanID())
		}

		switch e.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			out = append(out, e.IntValue())
		case pmetric.ExemplarValueTypeDouble:
			out = append(out, e.DoubleValue())
		default:
			require.Failf(t, "unexpected exemplar value type %v at index %d", e.ValueType().String(), i)
		}
	}
	return out
}
