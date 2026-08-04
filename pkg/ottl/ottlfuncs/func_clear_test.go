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

func Test_clear(t *testing.T) {
	var capturedValue any = "initial value"
	setterCalled := false

	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return capturedValue, nil
		},
		Setter: func(_ context.Context, _, val any) error {
			setterCalled = true
			capturedValue = val
			return nil
		},
	}

	exprFunc := clearFunc[any](target)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, setterCalled)
	assert.Empty(t, capturedValue, "clear should pass the zero-value string to the target setter")
}

func Test_clear_error_getter(t *testing.T) {
	expectedErr := errors.New("getter error")
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, expectedErr
		},
	}

	exprFunc := clearFunc[any](target)

	result, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error getting target value to infer zero value in clear")
	assert.Nil(t, result)
}

func Test_clear_error_setter(t *testing.T) {
	expectedErr := errors.New("setter error")
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "initial value", nil
		},
		Setter: func(_ context.Context, _, _ any) error {
			return expectedErr
		},
	}

	exprFunc := clearFunc[any](target)

	result, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result)
}
