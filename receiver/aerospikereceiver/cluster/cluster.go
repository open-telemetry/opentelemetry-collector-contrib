// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package cluster // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"

import (
	as "github.com/aerospike/aerospike-client-go/v5"
)

// asclient interface is for mocking
type asclient interface {
	GetNodes() []*as.Node
	Close()
}

// wrap aerospike Cluster so we can return node interfaces
type Cluster struct {
	conn asclient
}

func NewCluster(policy *as.ClientPolicy, hosts []*as.Host) (*Cluster, error) {
	c, err := as.NewClientWithPolicyAndHost(policy, hosts...)
	if err != nil {
		return nil, err
	}

	return &Cluster{c}, err
}

func (c *Cluster) GetNodes() []Node {
	asNodes := c.conn.GetNodes()
	nodes := make([]Node, len(asNodes))
	for i, n := range asNodes {
		nodes[i] = n
	}

	return nodes
}

func (c *Cluster) Close() {
	c.conn.Close()
}
