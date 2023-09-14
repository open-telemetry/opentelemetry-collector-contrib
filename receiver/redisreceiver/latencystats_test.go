// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLatencyStats(t *testing.T) {
	ls, err := parseLatencyStats("p50=181.247,p55=182.271,p99=309.247,p99.9=1023.999")
	require.Nil(t, err)
	require.Equal(t, ls["p50"], 181.247)
	require.Equal(t, ls["p55"], 182.271)
	require.Equal(t, ls["p99"], 309.247)
	require.Equal(t, ls["p99.9"], 1023.999)
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
			require.NotNil(t, err)
		})
	}
}
