// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_UUID(t *testing.T) {
	exprFunc, err := uuid[any]()
	require.NoError(t, err)

	value, err := exprFunc(nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, value)
}

func Test_UUIDFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewUUIDFactory[any]()
		assert.Equal(t, "UUID", factory.Name())
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewUUIDFactory[any]()
		fn, err := factory.CreateFunction(ottl.FunctionContext{}, nil)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})
}
