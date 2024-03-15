// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

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

func TestStrictRoundingDown(t *testing.T) {
	// The original hash function rounded thresholds down, in the
	// direction of zero.  The OTel hash function rounds
	// thresholds to the nearest value.  This slight difference is
	// controlled by the strict variable.

	// pct is approximately 75% of the minimum 14-bit probability, so it
	// will round up to 0x1p-14 unless strict, in which case it rounds
	// down to 0.
	const pct = 0x3p-16 * 100

	cfg := Config{
		SamplerMode:        HashSeed,
		SamplingPercentage: pct,
		HashSeed:           defaultHashSeed,
	}
	nostrict := commonFields{
		strict: false,
		logger: zaptest.NewLogger(t),
	}
	strict := commonFields{
		strict: true,
		logger: nostrict.logger,
	}

	// Rounds up in this case to the nearest/smallest 14-bit threshold.
	nostrictSamp, err := makeSampler(&cfg, nostrict)
	require.NoError(t, err)
	hasher, ok := nostrictSamp.(*hashingSampler)
	require.True(t, ok, "is non-zero")
	require.Equal(t, uint32(1), hasher.hashScaledSamplerate)

	// The strict behvaior rounds down in this case; makeSampler
	// returns a neverSampler.
	strictSamp, err := makeSampler(&cfg, strict)
	require.NoError(t, err)
	_, ok = strictSamp.(*neverSampler)
	require.True(t, ok, "is zero")
}
