// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer

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

	for _, suffix := range []string{"", ".otel-default"} {
		t.Run("suffix="+suffix, func(t *testing.T) {
			var pushedData []result
			push := func(count uint16, index string) error {
				pushedData = append(pushedData, result{index, count})
				return nil
			}

			// To make the expected data deterministic, seed the random number generator.
			// If the seed changes or the random number generator changes, this test will fail.
			rnd = rand.New(rand.NewPCG(0, 0))

			err := IndexDownsampledEvent(1000, DownsampledEventIndices(suffix), push)
			require.NoError(t, err)

			expectedData := []result{
				{"profiling-events-5pow01" + suffix, 201},
				{"profiling-events-5pow02" + suffix, 42},
				{"profiling-events-5pow03" + suffix, 9},
				{"profiling-events-5pow04" + suffix, 2},
				{"profiling-events-5pow05" + suffix, 1},
				{"profiling-events-5pow06" + suffix, 1},
			}

			require.Equal(t, expectedData, pushedData)
		})
	}
}

func TestDownsampledEventIndices(t *testing.T) {
	indices := DownsampledEventIndices(".otel-default")
	require.Len(t, indices, maxEventsIndexes)
	require.Equal(t, "profiling-events-5pow01.otel-default", indices[0])
	require.Equal(t, "profiling-events-5pow11.otel-default", indices[len(indices)-1])
}
