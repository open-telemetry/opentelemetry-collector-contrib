// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Unix(t *testing.T) {
	tests := []struct {
		name        string
		seconds     ottl.IntGetter[any]
		nanoseconds ottl.Optional[ottl.IntGetter[any]]
		expected    int64
	}{
		{
			name: "January 1, 2023",
			seconds: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(1672527600), nil
				},
			},
			nanoseconds: ottl.Optional[ottl.IntGetter[any]]{},
			expected:    int64(1672527600),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Unix(tt.seconds, tt.nanoseconds)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			want := time.Unix(tt.expected, 0)
			assert.Equal(t, want, result)
		})
	}
}
