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

func Test_isMatchVersion(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		constraint string
		expected   bool
		wantErr    bool
	}{
		{
			name:       "version match",
			value:      "1.2.3",
			constraint: "1.2.x",
			expected:   true,
			wantErr:    false,
		},
		{
			name:       "version doesn't match",
			value:      "1.3.3",
			constraint: "1.2.x",
			expected:   false,
			wantErr:    false,
		},
		{
			name:       "version pcommon.ValueTypeStr",
			value:      pcommon.NewValueStr("1.2.3"),
			constraint: "1.2.x",
			expected:   true,
			wantErr:    false,
		},
		{
			name:       "version pcommon.ValueTypeInt",
			value:      pcommon.NewValueInt(123),
			constraint: "1.2.x",
			expected:   false,
			wantErr:    true,
		},
		{
			name:       "not valid version string type",
			value:      "abc.2.3",
			constraint: "1.2.x",
			expected:   false,
			wantErr:    true,
		},
		{
			name:       "nil value",
			value:      nil,
			constraint: "1.2.x",
			expected:   false,
			wantErr:    true,
		},
		{
			name:       "not valid version pcommon.ValueTypeStr",
			value:      pcommon.NewValueStr("abc.2.3"),
			constraint: "1.2.x",
			expected:   false,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isMatchVersion[any](&ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.value, nil
				},
			}, tt.constraint)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)

			assert.True(t, (err != nil) == tt.wantErr, "Expected errors: %t received error: %t, err: %w", tt.wantErr, err != nil, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_isMatchVersion_invalid_constrain(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "1.2.3", nil
		},
	}
	_, err := isMatchVersion[any](target, "abc.1.2")
	assert.Error(t, err)
}
