// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package cluster // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"

import (
	as "github.com/aerospike/aerospike-client-go/v6"
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
