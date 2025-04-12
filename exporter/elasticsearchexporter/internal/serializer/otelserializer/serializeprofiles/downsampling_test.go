// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexDownsampledEvent(t *testing.T) {
	type result struct {
		index string
		count uint16
	}

	var pushedData []result
	pushData := func(data any, _, index string) error {
		pushedData = append(pushedData, result{index, data.(StackTraceEvent).Count})
		return nil
	}

	// To make the expected data deterministic, seed the random number generator.
	// If the seed changes or the random number generator changes, this test will fail.
	rnd = rand.New(rand.NewPCG(0, 0))

	err := IndexDownsampledEvent(StackTraceEvent{Count: 1000}, pushData)
	require.NoError(t, err)

	expectedData := []result{
		{"profiling-events-5pow01", 201},
		{"profiling-events-5pow02", 42},
		{"profiling-events-5pow03", 9},
		{"profiling-events-5pow04", 2},
		{"profiling-events-5pow05", 1},
		{"profiling-events-5pow06", 1},
	}

	require.Equal(t, expectedData, pushedData)
}
