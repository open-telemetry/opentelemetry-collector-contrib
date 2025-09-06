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
	type testCase struct {
		name string
		ctx  context.Context
		want any
		key  string
	}

	tests := []testCase{
		{
			name: "empty metadata",
			ctx:  nil, // Will be set to t.Context() in test
			want: nil,
			key:  "saw_metrics_tenant_id",
		},
		{
			name: "metadata with valid saw_metrics_tenant_id key",
			ctx:  nil, // Will be overridden in test
			want: "1548451",
			key:  "saw_metrics_tenant_id",
		},
		{
			name: "metadata with multiple values to saw_metrics_tenant_id key",
			ctx:  nil, // Will be overridden in test
			want: nil,
			key:  "saw_metrics_tenant_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			// Set up specific contexts for different test cases
			switch tt.name {
			case "metadata with valid saw_metrics_tenant_id key":
				cl := client.FromContext(t.Context())
				cl.Metadata = client.NewMetadata(
					map[string][]string{"saw_metrics_tenant_id": {"1548451"}},
				)
				ctx = client.NewContext(t.Context(), cl)
			case "metadata with multiple values to saw_metrics_tenant_id key":
				cl := client.FromContext(t.Context())
				cl.Metadata = client.NewMetadata(
					map[string][]string{"saw_metrics_tenant_id": {"1548451", "1548452"}},
				)
				ctx = client.NewContext(t.Context(), cl)
			}

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
