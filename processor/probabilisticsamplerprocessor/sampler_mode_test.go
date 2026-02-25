// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var AllModes = []SamplerMode{HashSeed, Equalizing, Proportional}

func TestUnmarshalText(t *testing.T) {
	tests := []struct {
		samplerMode string
		shouldError bool
	}{
		{
			samplerMode: "hash_seed",
		},
		{
			samplerMode: "equalizing",
		},
		{
			samplerMode: "proportional",
		},
		{
			samplerMode: "",
		},
		{
			samplerMode: "dunno",
			shouldError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.samplerMode, func(t *testing.T) {
			temp := modeUnset
			err := temp.UnmarshalText([]byte(tt.samplerMode))
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, temp, SamplerMode(tt.samplerMode))
		})
	}
}

func TestHashSeedRoundingDown(t *testing.T) {
	// The original hash function rounded thresholds down, in the
	// direction of zero.

	// pct is approximately 75% of the minimum 14-bit probability, so it
	// would round up, but it does not.
	const pct = 0x3p-16 * 100

	require.Equal(t, 1.0, math.Round((pct/100)*numHashBuckets))

	for _, isLogs := range []bool{false, true} {
		cfg := Config{
			Mode:               HashSeed,
			SamplingPercentage: pct,
			HashSeed:           defaultHashSeed,
		}

		_, ok := makeSampler(&cfg, isLogs).(*neverSampler)
		require.True(t, ok, "is neverSampler")
	}
}
