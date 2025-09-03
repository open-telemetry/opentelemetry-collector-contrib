package sawmillsfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestDdStatusRemapper(t *testing.T) {
	type testCase struct {
		name  string
		value any
		want  any
	}

	tests := []testCase{
		{
			name:  "nil value",
			value: nil,
			want:  nil,
		},
		// Numeric level tests
		{
			name:  "level 0",
			value: "0",
			want:  "emerg",
		},
		{
			name:  "level 1",
			value: "1",
			want:  "alert",
		},
		{
			name:  "level 2",
			value: "2",
			want:  "critical",
		},
		{
			name:  "level 3",
			value: "3",
			want:  "error",
		},
		{
			name:  "level 4",
			value: "4",
			want:  "warning",
		},
		{
			name:  "level 5",
			value: "5",
			want:  "notice",
		},
		{
			name:  "level 6",
			value: "6",
			want:  "info",
		},
		{
			name:  "level 7",
			value: "7",
			want:  "debug",
		},
		{
			name:  "level 8 (default)",
			value: "8",
			want:  "info",
		},

		// String level tests - emerg
		{
			name:  "emerg prefix",
			value: "emergency",
			want:  "emerg",
		},
		{
			name:  "f prefix",
			value: "fatal",
			want:  "emerg",
		},

		// String level tests - alert
		{
			name:  "a prefix",
			value: "alert",
			want:  "alert",
		},

		// String level tests - critical
		{
			name:  "c prefix",
			value: "critical",
			want:  "critical",
		},
		{
			name:  "c prefix capitalized",
			value: "Critical",
			want:  "critical",
		},

		// String level tests - error
		{
			name:  "e prefix",
			value: "error",
			want:  "error",
		},
		{
			name:  "err prefix",
			value: "err",
			want:  "error",
		},

		// String level tests - warning
		{
			name:  "w prefix",
			value: "warning",
			want:  "warning",
		},
		{
			name:  "warn prefix",
			value: "warn",
			want:  "warning",
		},

		// String level tests - notice
		{
			name:  "n prefix",
			value: "notice",
			want:  "notice",
		},

		// String level tests - info
		{
			name:  "i prefix",
			value: "info",
			want:  "info",
		},
		{
			name:  "information prefix",
			value: "information",
			want:  "info",
		},

		// String level tests - debug
		{
			name:  "d prefix",
			value: "debug",
			want:  "debug",
		},
		{
			name:  "trace prefix",
			value: "trace",
			want:  "debug",
		},
		{
			name:  "verbose prefix",
			value: "verbose",
			want:  "debug",
		},

		// String level tests - ok
		{
			name:  "o prefix",
			value: "ok",
			want:  "ok",
		},
		{
			name:  "s prefix",
			value: "success",
			want:  "ok",
		},
		{
			name:  "exactly ok",
			value: "ok",
			want:  "ok",
		},
		{
			name:  "exactly success",
			value: "success",
			want:  "ok",
		},

		// Empty string test
		{
			name:  "empty string",
			value: "",
			want:  "info",
		},

		// Default case test
		{
			name:  "unknown string",
			value: "unknown",
			want:  "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createDdStatusRemapperFunction[any](
				ottl.FunctionContext{},
				&DdStatusRemapperArguments[any]{
					Target: &ottl.StandardGetSetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.value, nil
						},
					},
				},
			)

			require.NoError(t, err)

			result, err := expressionFunc(nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}
