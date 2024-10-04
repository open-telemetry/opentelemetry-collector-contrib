// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_get(t *testing.T) {
	tests := []struct {
		name   string
		getter ottl.Getter[any]
		want   pcommon.Value
	}{
		{
			name: "get string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "a", nil
				},
			},
			want: pcommon.NewValueStr("a"),
		},
		{
			name: "get int",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 11, nil
				},
			},
			want: pcommon.NewValueInt(11),
		},
		{
			name: "get double",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 104.12, nil
				},
			},
			want: pcommon.NewValueDouble(104.12),
		},
		{
			name: "nil",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want: pcommon.NewValueEmpty(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := get(tt.getter)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			actual := pcommon.NewValueEmpty()
			assert.NoError(t, actual.FromRaw(result))
			assert.Equal(t, tt.want, actual)
		})
	}
}
