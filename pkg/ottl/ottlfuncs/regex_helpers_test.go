// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_newDynamicRegex_LiteralPattern(t *testing.T) {
	// Test with valid literal pattern
	getter := ottl.NewTestingLiteralGetter[any, string](true, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "test.*pattern", nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)
	require.NotNil(t, dr)
	require.NotNil(t, dr.value, "literal pattern should be pre-compiled")
	assert.Equal(t, "testFunc", dr.funcName)
	assert.Equal(t, "test.*pattern", dr.value.String())
}

func Test_newDynamicRegex_LiteralPattern_Invalid(t *testing.T) {
	// Test with invalid literal pattern
	getter := ottl.NewTestingLiteralGetter[any, string](true, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "[invalid(regex", nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	assert.Error(t, err)
	assert.Nil(t, dr)
	assert.Contains(t, err.Error(), "testFunc")
	assert.Contains(t, err.Error(), "[invalid(regex")
}

func Test_newDynamicRegex_DynamicPattern(t *testing.T) {
	// Test with dynamic pattern (non-literal getter)
	getter := ottl.NewTestingLiteralGetter[any, string](false, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "dynamic.*pattern", nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)
	require.NotNil(t, dr)
	assert.Nil(t, dr.value, "dynamic pattern should not be pre-compiled")
	assert.Equal(t, "testFunc", dr.funcName)
}

func Test_dynamicRegex_compile_Literal_NoOp(t *testing.T) {
	// Test that literal patterns are compiled once and compile() is a no-op
	pattern := "test.*pattern"
	getter := ottl.NewTestingLiteralGetter[any, string](true, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pattern, nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)
	require.NotNil(t, dr)
	require.NotNil(t, dr.value, "literal pattern should be pre-compiled")

	// Store the original compiled regex reference
	originalRegex := dr.value

	// Call compile multiple times
	ctx := t.Context()
	var tCtx any

	compiled1, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Same(t, originalRegex, compiled1, "first compile should return the same cached regex")

	compiled2, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Same(t, originalRegex, compiled2, "second compile should return the same cached regex")

	compiled3, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Same(t, originalRegex, compiled3, "third compile should return the same cached regex")

	// Verify all references are identical
	assert.Same(t, compiled1, compiled2)
	assert.Same(t, compiled2, compiled3)
	assert.Same(t, originalRegex, dr.value, "original cached value should not change")
}

func Test_dynamicRegex_compile_Dynamic_NewRegex(t *testing.T) {
	// Test that dynamic patterns return new compiled regexes each time
	pattern := "dynamic.*pattern"
	getter := ottl.NewTestingLiteralGetter[any, string](false, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pattern, nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)
	require.Nil(t, dr.value, "dynamic getter should not have pre-compiled value")

	ctx := t.Context()
	var tCtx any

	compiled1, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, compiled1)
	assert.Equal(t, pattern, compiled1.String())

	compiled2, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, compiled2)
	assert.Equal(t, pattern, compiled2.String())

	compiled3, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	require.NotNil(t, compiled3)
	assert.Equal(t, pattern, compiled3.String())

	// Verify that new regex instances are created each time
	assert.NotSame(t, compiled1, compiled2, "dynamic patterns should return new regex instances")
	assert.NotSame(t, compiled2, compiled3, "dynamic patterns should return new regex instances")
	assert.NotSame(t, compiled1, compiled3, "dynamic patterns should return new regex instances")
}

func Test_dynamicRegex_compile_Dynamic_InvalidPattern(t *testing.T) {
	// Test that invalid dynamic patterns return errors
	getter := ottl.NewTestingLiteralGetter[any, string](false, &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "[invalid(regex", nil
		},
	})

	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)
	require.Nil(t, dr.value)

	ctx := t.Context()
	var tCtx any

	compiled, err := dr.compile(ctx, tCtx)
	assert.Error(t, err)
	assert.Nil(t, compiled)
}

func Test_dynamicRegex_compile_Dynamic_GetterError(t *testing.T) {
	// Test that getter errors are properly propagated
	expectedErr := errors.New("getter error")
	errorGetter := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "", expectedErr
		},
	}
	dr, err := newDynamicRegex("testFunc", errorGetter)
	require.NoError(t, err)
	require.Nil(t, dr.value)

	ctx := t.Context()
	var tCtx any

	compiled, err := dr.compile(ctx, tCtx)
	assert.Error(t, err)
	assert.Nil(t, compiled)
	assert.Contains(t, err.Error(), "testFunc")
}

func Test_dynamicRegex_compile_Dynamic_DifferentPatterns(t *testing.T) {
	// Test that different dynamic patterns are compiled correctly
	patterns := []string{"pattern1.*", "pattern2.*", "pattern3.*"}
	currentPattern := 0

	getter := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			pattern := patterns[currentPattern]
			currentPattern = (currentPattern + 1) % len(patterns)
			return pattern, nil
		},
	}
	dr, err := newDynamicRegex("testFunc", getter)
	require.NoError(t, err)

	ctx := t.Context()
	var tCtx any

	// Compile with different patterns and verify each is compiled correctly
	compiled1, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Equal(t, patterns[0], compiled1.String())

	compiled2, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Equal(t, patterns[1], compiled2.String())

	compiled3, err := dr.compile(ctx, tCtx)
	require.NoError(t, err)
	assert.Equal(t, patterns[2], compiled3.String())

	// Verify that new regex instances are created for different patterns
	assert.NotSame(t, compiled1, compiled2, "different patterns should create new regex instances")
	assert.NotSame(t, compiled2, compiled3, "different patterns should create new regex instances")
	assert.NotSame(t, compiled1, compiled3, "different patterns should create new regex instances")
}
