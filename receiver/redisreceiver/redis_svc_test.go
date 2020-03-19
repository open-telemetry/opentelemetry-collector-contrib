package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newFakeApiParser() *redisSvc {
	return newRedisSvc(fakeClient{})
}

func TestParser(t *testing.T) {
	s := newFakeApiParser()
	info, err := s.info()
	require.Nil(t, err)
	require.Equal(t, 123, len(info))
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check
}
