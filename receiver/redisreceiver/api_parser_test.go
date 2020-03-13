package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newFakeApiService() *apiParser {
	return newApiParser(fakeClient{})
}

func TestService_ServerInfo(t *testing.T) {
	s := newFakeApiService()
	info, err := s.info()
	require.Nil(t, err)
	require.Equal(t, 121, len(info))
	require.Equal(t, "1.24", info["allocator_frag_ratio"]) // spot check
}
