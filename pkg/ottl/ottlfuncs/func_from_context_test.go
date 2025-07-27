// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package ottlfuncs

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
)

func TestFromContext(t *testing.T) {
	type testCase struct {
		name string
		ctx  func() context.Context
		want any
		key  string
	}

	tests := []testCase{
		{
			name: "empty metadata",
			ctx: func() context.Context {
				return context.Background()
			},
			want: nil,
			key:  "tenant_id",
		},
		{
			name: "metadata with valid tenant_id key",
			ctx: func() context.Context {
				cl := client.FromContext(context.Background())
				cl.Metadata = client.NewMetadata(
					map[string][]string{"tenant_id": {"1548451"}},
				)
				return client.NewContext(context.Background(), cl)
			},
			want: "1548451",
			key:  "tenant_id",
		},
		{
			name: "metadata with multiple values to tenant_id key",
			ctx: func() context.Context {
				cl := client.FromContext(context.Background())
				cl.Metadata = client.NewMetadata(
					map[string][]string{"tenant_id": {"1548451", "1548452"}},
				)
				return client.NewContext(context.Background(), cl)
			},
			want: nil,
			key:  "tenant_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx()
			expressionFunc, err := createFromContextFunction[any](
				ottl.FunctionContext{},
				&FromContextArguments[any]{
					Key: tt.key,
				},
			)

			require.NoError(t, err)

			result, err := expressionFunc(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
