// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cluster // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"

import (
	"testing"

	as "github.com/aerospike/aerospike-client-go/v6"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster/mocks"
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
