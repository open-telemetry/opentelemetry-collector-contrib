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

type Node interface {
	RequestInfo(*as.InfoPolicy, ...string) (map[string]string, as.Error)
	GetName() string
	Close()
}

// ConnNode is for single node scraping
type ConnNode struct {
	conn *as.Connection
	name string
}

func NewConnNode(policy *as.ClientPolicy, host *as.Host, authEnabled bool) (*ConnNode, error) {
	conn, err := as.NewConnection(policy, host)
	if err != nil {
		return nil, err
	}

	if authEnabled {
		if err := conn.Login(policy); err != nil {
			return nil, err
		}
	}

	m, err := conn.RequestInfo("node")
	if err != nil {
		return nil, err
	}
	name := m["node"]

	res := ConnNode{
		conn: conn,
		name: name,
	}

	return &res, nil
}

func (n *ConnNode) RequestInfo(_ *as.InfoPolicy, commands ...string) (map[string]string, as.Error) {
	res, err := n.conn.RequestInfo(commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (n *ConnNode) GetName() string {
	return n.name
}

func (n *ConnNode) Close() {
	n.conn.Close()
}
