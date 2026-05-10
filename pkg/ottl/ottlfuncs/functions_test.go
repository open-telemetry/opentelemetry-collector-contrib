// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStandardRegistriesIncludeSawmillsFactories(t *testing.T) {
	expected := []string{
		"DdStatusRemapper",
		"Contains",
		"EndsWith",
		"StartsWith",
		"IsInRange",
		"FromContext",
	}

	converters := StandardConverters[any]()
	functions := StandardFuncs[any]()
	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			converter, ok := converters[name]
			require.True(t, ok)
			require.Equal(t, name, converter.Name())

			function, ok := functions[name]
			require.True(t, ok)
			require.Equal(t, name, function.Name())
		})
	}

	_, ok := converters["split_metric_by_attributes"]
	require.False(t, ok)

	splitMetric, ok := functions["split_metric_by_attributes"]
	require.True(t, ok)
	require.Equal(t, "split_metric_by_attributes", splitMetric.Name())
}
