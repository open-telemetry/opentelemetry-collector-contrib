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
	require.Equal(t, 3, ks.avgTtl)
}
