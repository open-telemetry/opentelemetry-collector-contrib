// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLatencyStats(t *testing.T) {
	ls, err := parseLatencyStats("p50=181.247,p55=182.271,p99=309.247,p99.9=1023.999")
	require.NoError(t, err)
	require.Equal(t, 181.247, ls["p50"])
	require.Equal(t, 182.271, ls["p55"])
	require.Equal(t, 309.247, ls["p99"])
	require.Equal(t, 1023.999, ls["p99.9"])
}

func TestParseMalformedLatencyStats(t *testing.T) {
	tests := []struct{ name, stats string }{
		{"missing value", "p50=42.0,p90=50.0,p99.9="},
		{"missing equals", "p50=42.0,p90=50.0,p99.9"},
		{"extra comma", "p50=42.0,,p90=50.0"},
		{"wrong value type", "p50=asdf"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseLatencyStats(test.stats)
			require.Error(t, err)
		})
	}
}
