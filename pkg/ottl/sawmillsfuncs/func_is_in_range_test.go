// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestIsInRange(t *testing.T) {
	tests := []struct {
		name    string
		target  ottl.FloatLikeGetter[any]
		min     ottl.FloatLikeGetter[any]
		max     ottl.FloatLikeGetter[any]
		want    bool
		wantErr string
	}{
		{
			name: "int in range",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			want: true,
		},
		{
			name: "int equal to min",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			want: true,
		},
		{
			name: "int equal to max",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			want: true,
		},
		{
			name: "int below min",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(0), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			want: false,
		},
		{
			name: "int above max",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(11), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			want: false,
		},
		{
			name: "float in range",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(5.5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: true,
		},
		{
			name: "float equal to min",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: true,
		},
		{
			name: "float equal to max",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: true,
		},
		{
			name: "float below min",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.0), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: false,
		},
		{
			name: "float above max",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.2), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: false,
		},
		{
			name: "mixed int and float",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: true,
		},
		{
			name: "valid string number in range",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "5.5", nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: true,
		},
		{
			name: "valid string number below range",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "1.0", nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: false,
		},
		{
			name: "valid string number above range",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "11", nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(1.1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return float64(10.1), nil },
			},
			want: false,
		},
		{
			name: "string not a number",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "not a number", nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			wantErr: "target must be a number",
		},
		{
			name: "min not a number",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "not a number", nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			wantErr: "min must be a number",
		},
		{
			name: "max not a number",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return "not a number", nil },
			},
			wantErr: "max must be a number",
		},
		{
			name: "min greater than max",
			target: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(5), nil },
			},
			min: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(10), nil },
			},
			max: ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) { return int64(1), nil },
			},
			wantErr: "min must be less than or equal to max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			function, err := createIsInRangeFunction[any](
				ottl.FunctionContext{
					Set: componenttest.NewNopTelemetrySettings(),
				},
				&IsInRangeArguments[any]{
					Target: tt.target,
					Min:    tt.min,
					Max:    tt.max,
				},
			)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)

			result, err := function(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
