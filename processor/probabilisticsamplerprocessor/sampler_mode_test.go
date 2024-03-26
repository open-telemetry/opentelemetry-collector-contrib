// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
