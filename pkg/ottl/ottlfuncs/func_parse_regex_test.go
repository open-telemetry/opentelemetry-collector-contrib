package ottlfuncs

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Test_ParseRegex(t *testing.T) {

	target := &ottl.StandardStringGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (interface{}, error) {
			return `{"test":"string value"}`, nil
		},
	}
	tests := []struct {
		name    string
		target  ottl.StringGetter[any]
		pattern string
		want    func(pcommon.Value)
	}{
		{
			name:    "parse regex",
			target:  target,
			pattern: "",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := parseRegex(tt.target, tt.pattern)
			assert.NoError(t, err)
			assert.NotNil(t, exprFunc)

		})
	}
}
