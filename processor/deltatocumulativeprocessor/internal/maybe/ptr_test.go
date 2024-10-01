// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maybe_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maybe"
)

func TestMaybe(t *testing.T) {
	t.Run("zero-not-ok", func(t *testing.T) {
		var ptr maybe.Ptr[int]
		_, ok := ptr.Try()
		require.False(t, ok)
	})
	t.Run("none-not-ok", func(t *testing.T) {
		ptr := maybe.None[int]()
		_, ok := ptr.Try()
		require.False(t, ok)
	})
	t.Run("explicit-nil", func(t *testing.T) {
		ptr := maybe.Some[int](nil)
		v, ok := ptr.Try()
		require.Nil(t, v)
		require.True(t, ok)
	})
	t.Run("value", func(t *testing.T) {
		num := 42
		ptr := maybe.Some(&num)
		v, ok := ptr.Try()
		require.True(t, ok)
		require.Equal(t, num, *v)
	})
}

func ExamplePtr() {
	var unset maybe.Ptr[int] // = maybe.None()
	if v, ok := unset.Try(); ok {
		fmt.Println("unset:", v)
	} else {
		fmt.Println("unset: !ok")
	}

	var xnil maybe.Ptr[int] = maybe.Some[int](nil)
	if v, ok := xnil.Try(); ok {
		fmt.Println("explicit nil:", v)
	}

	num := 42
	var set maybe.Ptr[int] = maybe.Some(&num)
	if v, ok := set.Try(); ok {
		fmt.Println("set:", *v)
	}

	// Output:
	// unset: !ok
	// explicit nil: <nil>
	// set: 42
}
