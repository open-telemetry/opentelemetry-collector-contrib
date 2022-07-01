package cluster

import (
	"testing"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster/mocks"
	"github.com/stretchr/testify/require"
)

func TestCluster_GetNodes(t *testing.T) {
	t.Parallel()

	asc := mocks.NewAsclient(t)
	nodes := []*as.Node{
		{},
		{},
	}
	asc.On("GetNodes").Return(nodes)

	testCluster := Cluster{
		conn: asc,
	}

	actualNodes := testCluster.GetNodes()
	require.Equal(t, len(actualNodes), len(nodes))
}
