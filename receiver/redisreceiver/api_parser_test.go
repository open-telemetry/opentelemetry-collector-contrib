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
	all, err := s.info()
	require.Nil(t, err)
	require.Equal(t, 121, len(all))
	require.Equal(t, "1.24", all["allocator_frag_ratio"])
}
