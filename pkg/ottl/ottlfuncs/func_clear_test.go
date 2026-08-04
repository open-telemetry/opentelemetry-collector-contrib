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
	assert.Nil(t, capturedValue, "clear should pass nil to the target setter")
}

func Test_clear_error(t *testing.T) {
	expectedErr := errors.New("setter error")
	target := &ottl.StandardGetSetter[any]{
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
