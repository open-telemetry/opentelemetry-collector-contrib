// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package cluster // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"

import (
	"fmt"
	"strings"
	"time"

	as "github.com/aerospike/aerospike-client-go/v6"
	"github.com/aerospike/aerospike-client-go/v6/types"
)

// asconn is used to mock aerospike connections
type asconn interface {
	RequestInfo(...string) (map[string]string, as.Error)
	Login(*as.ClientPolicy) as.Error
	Close()
	SetTimeout(time.Time, time.Duration) as.Error
}

type Node interface {
	RequestInfo(*as.InfoPolicy, ...string) (map[string]string, as.Error)
	GetName() string
	Close()
}

// connNode is for single node scraping
type connNode struct {
	conn   asconn
	policy *as.ClientPolicy
	name   string
}

type connFactoryFunc func(*as.ClientPolicy, *as.Host) (asconn, as.Error)

func newASConn(policy *as.ClientPolicy, host *as.Host) (asconn, as.Error) {
	return as.NewConnection(policy, host)
}

func newConnNode(policy *as.ClientPolicy, host *as.Host, authEnabled bool) (Node, error) {
	return _newConnNode(policy, host, authEnabled, newASConn)
}

func _newConnNode(policy *as.ClientPolicy, host *as.Host, authEnabled bool, connF connFactoryFunc) (Node, error) {
	conn, err := connF(policy, host)
	if err != nil {
		return nil, err
	}

	var deadline time.Time
	// Set deadline to 0 (inf) so we can always reuse this connection
	if err = conn.SetTimeout(deadline, policy.Timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}

	if authEnabled {
		if err = conn.Login(policy); err != nil {
			return nil, err
		}
	}

	m, err := conn.RequestInfo("node")
	if err != nil {
		return nil, err
	}

	for k := range m {
		if strings.HasPrefix(strings.ToUpper(k), "ERROR:") {
			return nil, as.ErrNotAuthenticated
		}
	}

	name := m["node"]

	res := connNode{
		conn:   conn,
		policy: policy,
		name:   name,
	}

	return &res, nil
}

func (n *connNode) RequestInfo(_ *as.InfoPolicy, commands ...string) (map[string]string, as.Error) {
	res, err := n.conn.RequestInfo(commands...)
	// Try to login and get a new session
	if err != nil && err.Matches(types.EXPIRED_SESSION) {
		if loginErr := n.conn.Login(n.policy); loginErr != nil {
			return nil, loginErr
		}
	}

	if err != nil {
		return nil, err
	}

	for k := range res {
		if strings.HasPrefix(strings.ToUpper(k), "ERROR:") {
			return nil, as.ErrNotAuthenticated
		}
	}

	return res, nil
}

func (n *connNode) GetName() string {
	return n.name
}

func (n *connNode) Close() {
	n.conn.Close()
}
