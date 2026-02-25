// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_copyMetric(t *testing.T) {
	tests := []struct {
		testName string
		name     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]
		desc     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]
		unit     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]
		want     func(s pmetric.MetricSlice)
	}{
		{
			testName: "basic copy",
			name:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			desc:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			unit:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			want: func(ms pmetric.MetricSlice) {
				metric := ms.At(0)
				newMetric := ms.AppendEmpty()
				metric.CopyTo(newMetric)
			},
		},
		{
			testName: "set name",
			name: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new name", nil
				},
			}),
			desc: ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			unit: ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			want: func(ms pmetric.MetricSlice) {
				metric := ms.At(0)
				newMetric := ms.AppendEmpty()
				metric.CopyTo(newMetric)
				newMetric.SetName("new name")
			},
		},
		{
			testName: "set description",
			name:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			desc: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new desc", nil
				},
			}),
			unit: ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			want: func(ms pmetric.MetricSlice) {
				metric := ms.At(0)
				newMetric := ms.AppendEmpty()
				metric.CopyTo(newMetric)
				newMetric.SetDescription("new desc")
			},
		},
		{
			testName: "set unit",
			name:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			desc:     ottl.Optional[ottl.StringGetter[*ottlmetric.TransformContext]]{},
			unit: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new unit", nil
				},
			}),
			want: func(ms pmetric.MetricSlice) {
				metric := ms.At(0)
				newMetric := ms.AppendEmpty()
				metric.CopyTo(newMetric)
				newMetric.SetUnit("new unit")
			},
		},
		{
			testName: "set all",
			name: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new name", nil
				},
			}),
			desc: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new desc", nil
				},
			}),
			unit: ottl.NewTestingOptional[ottl.StringGetter[*ottlmetric.TransformContext]](ottl.StandardStringGetter[*ottlmetric.TransformContext]{
				Getter: func(context.Context, *ottlmetric.TransformContext) (any, error) {
					return "new unit", nil
				},
			}),
			want: func(ms pmetric.MetricSlice) {
				metric := ms.At(0)
				newMetric := ms.AppendEmpty()
				metric.CopyTo(newMetric)
				newMetric.SetName("new name")
				newMetric.SetDescription("new desc")
				newMetric.SetUnit("new unit")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			ms := pmetric.NewScopeMetrics()
			input := ms.Metrics().AppendEmpty()
			input.SetName("test")
			input.SetDescription("test")
			input.SetUnit("test")
			input.SetEmptySum()
			d := input.Sum().DataPoints().AppendEmpty()
			d.SetIntValue(1)

			expected := pmetric.NewScopeMetrics()
			ms.Metrics().CopyTo(expected.Metrics())
			tt.want(expected.Metrics())

			exprFunc, err := copyMetric(tt.name, tt.desc, tt.unit)
			require.NoError(t, err)
			tCtx := ottlmetric.NewTransformContextPtr(pmetric.NewResourceMetrics(), ms, input)
			defer tCtx.Close()
			_, err = exprFunc(t.Context(), tCtx)
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareScopeMetrics(expected, ms))
		})
	}
}
