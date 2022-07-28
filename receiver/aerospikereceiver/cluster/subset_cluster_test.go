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
	"errors"
	"testing"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster/mocks"
)

func TestSubsetCluster_New(t *testing.T) {
	t.Parallel()

	nodes := []Node{
		mocks.NewNode(t),
		mocks.NewNode(t),
	}

	cPolicy := as.NewClientPolicy()
	hosts := []*as.Host{
		as.NewHost("testip", 3000),
		as.NewHost("testip", 4000),
	}
	authEnabled := true

	nodeFactoryPos := newMockNodeFactoryFunc(t)
	nodeFactoryPos.On("Execute", cPolicy, hosts[0], authEnabled).Return(nodes[0], nil)
	nodeFactoryPos.On("Execute", cPolicy, hosts[1], authEnabled).Return(nodes[1], nil)

	factoryFuncPos := func(policy *as.ClientPolicy, hosts *as.Host, authEnabled bool) (Node, error) {
		return nodeFactoryPos.Execute(policy, hosts, authEnabled)
	}

	nodeFactoryNeg := newMockNodeFactoryFunc(t)
	nodeFactoryNeg.On("Execute", cPolicy, hosts[0], authEnabled).Return(nodes[0], nil)
	nodeFactoryNeg.On("Execute", cPolicy, hosts[1], authEnabled).Return(nil, errors.New("invalid host"))

	factoryFuncNeg := func(policy *as.ClientPolicy, hosts *as.Host, authEnabled bool) (Node, error) {
		return nodeFactoryNeg.Execute(policy, hosts, authEnabled)
	}

	testCluster, err := newSubsetCluster(cPolicy, hosts, authEnabled, factoryFuncPos)
	require.NoError(t, err)
	nodeFactoryPos.AssertExpectations(t)
	require.Equal(t, len(testCluster.GetNodes()), len(nodes))

	_, err = newSubsetCluster(cPolicy, hosts, authEnabled, factoryFuncNeg)
	nodeFactoryNeg.AssertExpectations(t)
	require.EqualError(t, err, "invalid host")
}

func TestSubsetCluster_GetNodes(t *testing.T) {
	t.Parallel()

	nodes := []Node{
		mocks.NewNode(t),
		mocks.NewNode(t),
	}

	testCluster := SubsetCluster{
		nodes: nodes,
	}

	actualNodes := testCluster.GetNodes()
	require.Equal(t, len(actualNodes), len(nodes))
}

func TestSubsetCluster_Close(t *testing.T) {
	t.Parallel()

	n1 := mocks.NewNode(t)
	n1.On("Close").Return()

	n2 := mocks.NewNode(t)
	n2.On("Close").Return()

	nodes := []Node{
		n1,
		n2,
	}

	testCluster := SubsetCluster{
		nodes: nodes,
	}

	testCluster.Close()
	n1.AssertExpectations(t)
	n2.AssertExpectations(t)
}
