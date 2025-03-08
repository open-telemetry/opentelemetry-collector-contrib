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
		name        string
		target      ottl.StringGetter[any]
		replacement ottl.Optional[string]
		expected    any
		shouldError bool
	}{
		{
			name: "trim string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](" "),
			expected:    "this is a test",
			shouldError: false,
		},
		{
			name: "trim empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](" "),
			expected:    "",
			shouldError: false,
		},
		{
			name: "No replacement string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.Optional[string]{},
			expected:    "this is a test",
			shouldError: false,
		},
		{
			name: "Set replacement string to \"\"",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](""),
			expected:    " this is a test ",
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := trim(tt.target, tt.replacement)
			result, err := exprFunc(nil, nil)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
