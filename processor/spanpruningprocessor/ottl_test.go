// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestStandardSpanFuncs(t *testing.T) {
	funcs := standardSpanFuncs()

	require.NotNil(t, funcs)
	assert.NotEmpty(t, funcs)

	// Verify IsRootSpan function is included.
	_, hasIsRootSpan := funcs["IsRootSpan"]
	assert.True(t, hasIsRootSpan, "standardSpanFuncs should include IsRootSpan")

	// Verify some standard converters are present.
	expectedFuncs := []string{
		"Concat",
		"Int",
		"String",
	}
	for _, name := range expectedFuncs {
		_, ok := funcs[name]
		assert.True(t, ok, "standardSpanFuncs should include %s", name)
	}
}

func TestNewBoolExprForSpan(t *testing.T) {
	tests := []struct {
		name        string
		conditions  []string
		errorMode   ottl.ErrorMode
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid simple condition",
			conditions:  []string{`name == "test"`},
			errorMode:   ottl.PropagateError,
			expectError: false,
		},
		{
			name:        "valid IsRootSpan condition",
			conditions:  []string{`IsRootSpan()`},
			errorMode:   ottl.PropagateError,
			expectError: false,
		},
		{
			name:        "valid multiple conditions",
			conditions:  []string{`name == "test"`, `status.code == 2`},
			errorMode:   ottl.IgnoreError,
			expectError: false,
		},
		{
			name:        "invalid condition syntax",
			conditions:  []string{`invalid syntax here`},
			errorMode:   ottl.PropagateError,
			expectError: true,
		},
		{
			name:        "unknown function",
			conditions:  []string{`UnknownFunction() == true`},
			errorMode:   ottl.PropagateError,
			expectError: true,
		},
		{
			name:        "empty conditions",
			conditions:  []string{},
			errorMode:   ottl.PropagateError,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := standardSpanFuncs()
			settings := componenttest.NewNopTelemetrySettings()

			result, err := newBoolExprForSpan(tt.conditions, funcs, tt.errorMode, settings)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
