// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_trim(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		expected any
	}{
		{
			name: "trim string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return " this is a test ", nil
				},
			},
			expected: "this is a test",
		},
		{
			name: "trim empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "", nil
				},
			},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := trim(tt.target)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
