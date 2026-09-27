// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func TestStandardFunctions(t *testing.T) {
	assertStandardFunctions(t, standardFunctions[any]())
}

func TestSpanFunctions(t *testing.T) {
	funcs := spanFunctions()
	assertStandardFunctions(t, funcs)

	isRouteFuncName := ottlfuncs.NewIsRootSpanFactory().Name()
	val, ok := funcs[isRouteFuncName]

	require.True(t, ok)
	require.NotNil(t, val)
	require.Equal(t, isRouteFuncName, val.Name())
}

func assertStandardFunctions[K any](t *testing.T, funcs map[string]ottl.Factory[K]) {
	expectedFunctions := slices.AppendSeq([]string{
		ottlfuncs.NewDeleteKeyFactory[any]().Name(),
		ottlfuncs.NewDeleteMatchingKeysFactory[any]().Name(),
		"route",
	}, maps.Keys(ottlfuncs.StandardConverters[K]()))

	for _, fn := range expectedFunctions {
		val, ok := funcs[fn]
		require.True(t, ok, "Function %s not found in standard functions", fn)
		require.NotNil(t, val, "Function %s should not be nil", fn)
		require.Equal(t, fn, val.Name(), "Function name mismatch for %s", fn)
	}
}
