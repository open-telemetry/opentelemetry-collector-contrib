// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseKeyspace(t *testing.T) {
	ks, err := parseKeyspaceString(9, "keys=1,expires=2,avg_ttl=3")
	require.Nil(t, err)
	require.Equal(t, "9", ks.db)
	require.Equal(t, 1, ks.keys)
	require.Equal(t, 2, ks.expires)
	require.Equal(t, 3, ks.avgTTL)
}

func TestParseMalformedKeyspace(t *testing.T) {
	tests := []struct{ name, keyspace string }{
		{"missing value", "keys=1,expires=2,avg_ttl="},
		{"missing equals", "keys=1,expires=2,avg_ttl"},
		{"unexpected key", "xyz,keys=1,expires=2"},
		{"no usable data", "foo"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseKeyspaceString(0, test.keyspace)
			require.NotNil(t, err)
		})
	}
}
