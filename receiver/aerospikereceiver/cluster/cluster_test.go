// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
