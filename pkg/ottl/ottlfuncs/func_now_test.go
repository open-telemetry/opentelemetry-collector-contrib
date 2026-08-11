// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Now(t *testing.T) {
	exprFunc, err := now[any]()
	require.NoError(t, err)

	value, err := exprFunc(nil, nil)
	require.NoError(t, err)
	// There should be basically no difference between the value of time.Now() returned by the ottlfunc vs time.Now() run in the test.
	n := time.Now()
	assert.LessOrEqual(t, n.Sub(value.(time.Time)).Seconds(), 1.0)
}

func Test_NowFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewNowFactory[any]()
		assert.Equal(t, "Now", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewNowFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.Nil(t, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewNowFactory[any]()

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, factory.CreateDefaultArguments())
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})
}
