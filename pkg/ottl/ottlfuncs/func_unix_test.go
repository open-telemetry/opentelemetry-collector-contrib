package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Unix(t *testing.T) {
	tests := []struct {
		name        string
		seconds     ottl.IntGetter[any]
		nanoseconds ottl.Optional[ottl.IntGetter[any]]
		expected    time.Time
	}{
		{
			name: "January 1, 2023",
			seconds: &ottl.StandardIntGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return int64(1672527600), nil
				},
			},
			nanoseconds: ottl.Optional[ottl.IntGetter[any]]{},
			expected:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Unix(tt.seconds, tt.nanoseconds)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
