// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  func(*testing.T) context.Context
		want any
		key  string
	}{
		{
			name: "empty metadata",
			ctx: func(t *testing.T) context.Context {
				return t.Context()
			},
			want: nil,
			key:  "saw_metrics_tenant_id",
		},
		{
			name: "metadata with valid saw_metrics_tenant_id key",
			ctx: func(t *testing.T) context.Context {
				ctx := t.Context()
				cl := client.FromContext(ctx)
				cl.Metadata = client.NewMetadata(
					map[string][]string{"saw_metrics_tenant_id": {"1548451"}},
				)
				return client.NewContext(ctx, cl)
			},
			want: "1548451",
			key:  "saw_metrics_tenant_id",
		},
		{
			name: "metadata with multiple values to saw_metrics_tenant_id key",
			ctx: func(t *testing.T) context.Context {
				ctx := t.Context()
				cl := client.FromContext(ctx)
				cl.Metadata = client.NewMetadata(
					map[string][]string{"saw_metrics_tenant_id": {"1548451", "1548452"}},
				)
				return client.NewContext(ctx, cl)
			},
			want: nil,
			key:  "saw_metrics_tenant_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx(t)
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
