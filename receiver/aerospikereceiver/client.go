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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"fmt"
	"strings"
	"sync"
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/aerospike/aerospike-client-go/v5/types"
	"go.uber.org/zap"
)

type clientData struct {
	policy *as.ClientPolicy // Timeout and authentication information
	logger *zap.Logger      // logs malformed metrics in responses
}

type defaultASClient struct {
	conn *as.Connection // open connection to Aerospike
	clientData
}

type clusterASClient struct {
	cluster *as.Cluster
	clientData
}

var defaultNodeInfoCommands = []string{
	"namespaces",
	"node",
	"statistics",
	"services",
}

// newASClient creates a new defaultASClient connected to the given host and port.
// If username and password aren't blank, they're used to authenticate
func newClusterASClient(host string, port int, username, password string, timeout time.Duration, logger *zap.Logger) (*clusterASClient, error) {
	authEnabled := username != "" && password != ""

	policy := as.NewClientPolicy()
	policy.Timeout = timeout
	if authEnabled {
		policy.User = username
		policy.Password = password
	}

	hosts := []*as.Host{as.NewHost(host, port)}
	cluster, err := as.NewCluster(policy, hosts) // as.NewConnection(policy, as.NewHost(host, port))
	if err != nil {
		return nil, err
	}

	return &clusterASClient{
		cluster: cluster,
		clientData: clientData{
			logger: logger,
			policy: policy,
		},
	}, nil
}

// the info map has layers map[string]map[string]string
// map[string]map[string]string{
//		nodeName: map[string]string {
//			infoCommandName: statsString
// 		}
// }
type clusterInfo map[string]map[string]string // TODO what is this is connected to multiple clusters?
type metricsMap map[string]string

// func (c *clusterASClient) Info() clusterInfo {
// 	nodes := c.cluster.GetNodes()
// 	var res clusterInfo

// 	policy := as.NewInfoPolicy()
// 	policy.Timeout = c.policy.Timeout

// 	res = mapNodesInfoRequest(
// 		nodes,
// 		defaultNodeInfoCommands,
// 		policy,
// 		c.logger,
// 	)

// 	return res
// }

type nodeStats struct {
	name    string
	metrics metricsMap
	error   error
}

type nodeFunc func(n node, policy *as.InfoPolicy) (metricsMap, error)

func mapNodeInfoFunc(nodes []node, nf nodeFunc, policy *as.InfoPolicy) clusterInfo {
	numNodes := len(nodes)
	res := make(clusterInfo, numNodes)

	var wg sync.WaitGroup
	resChan := make(chan nodeStats, numNodes)

	for _, nd := range nodes {
		wg.Add(1)
		go func(nd node) {
			defer wg.Done()

			name := nd.GetName()
			m, err := nf(nd, policy)
			// if err != nil {
			// 	logger.Sugar().Errorf("mapNodesInfoRequest info err: %w", err)
			// } TODO handle errors

			ns := nodeStats{
				name:    name,
				metrics: m,
				error:   err,
			}

			resChan <- ns
		}(nd)
	}

	wg.Wait()
	close(resChan)

	for ns := range resChan {
		res[ns.name] = ns.metrics
	}

	return res
}

// func (c *clusterASClient) nodeNamespaces() clusterInfo {
// 	nodes := c.cluster.GetNodes()
// 	commands := []string{"namespaces"}

// 	var res clusterInfo

// 	policy := as.NewInfoPolicy() // TODO create the policy once somewhere else
// 	policy.Timeout = c.policy.Timeout

// 	res = mapNodesInfoRequest(
// 		nodes,
// 		commands,
// 		policy,
// 		c.logger,
// 	) // todo maybe check errors in res

// 	return res
// }

type node interface {
	RequestInfo(*as.InfoPolicy, ...string) (map[string]string, error)
	GetName() string
}

func namespaceNames(n node, policy *as.InfoPolicy) ([]string, error) {
	var namespaces []string

	info, err := n.RequestInfo(policy, "namespaces")
	if err != nil {
		return nil, err // TODO maybe add something via partial error
	}

	namespaces = strings.Split(info["namespaces"], ";")
	return namespaces, err
}

func allNodeInfo(n node, policy *as.InfoPolicy) (metricsMap, error) {
	var res metricsMap
	commands := defaultNodeInfoCommands

	res, err := n.RequestInfo(policy, commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func allNamespaceInfo(n node, policy *as.InfoPolicy) (metricsMap, error) {
	var res metricsMap

	names, err := namespaceNames(n, policy)
	if err != nil {
		return nil, err
	}

	commands := make([]string, len(names))
	for i, name := range names {
		commands[i] = fmt.Sprintf("namespace/%s", name)
	}

	res, err = n.RequestInfo(policy, commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// aerospike is the interface that provides information about a given node
type aerospike interface {
	// NamespaceInfo gets information about a specific namespace
	NamespaceInfo(namespace string) (map[string]string, error)
	// Info gets high-level information about the node/system.
	Info() (map[string]string, error)
	// Close closes the connection to the Aerospike node
	Close()
}

// newASClient creates a new defaultASClient connected to the given host and port.
// If username and password aren't blank, they're used to authenticate
func newASClient(host string, port int, username, password string, timeout time.Duration, logger *zap.Logger) (*defaultASClient, error) {
	authEnabled := username != "" && password != ""

	policy := as.NewClientPolicy()
	policy.Timeout = timeout
	if authEnabled {
		policy.User = username
		policy.Password = password
	}

	conn, err := as.NewConnection(policy, as.NewHost(host, port))
	if err != nil {
		return nil, err
	}

	if authEnabled {
		if err := conn.Login(policy); err != nil {
			return nil, err
		}
	}

	return &defaultASClient{
		conn: conn,
		clientData: clientData{
			logger: logger,
			policy: policy,
		},
	}, nil
}

func (c *defaultASClient) NamespaceInfo(namespace string) (map[string]string, error) {
	// is this using the deadline correctly?
	if err := c.conn.SetTimeout(time.Now().Add(c.policy.Timeout), c.policy.Timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}
	namespaceKey := "namespace/" + namespace
	response, err := c.conn.RequestInfo(namespaceKey)

	// Try to login and get a new session
	if err != nil && err.Matches(types.EXPIRED_SESSION) {
		if loginErr := c.conn.Login(c.policy); loginErr != nil {
			return nil, loginErr
		}
	}

	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	for k, v := range response {
		if k == namespaceKey {
			for _, pair := range splitFields(v) {
				parts := splitPair(pair)
				if len(parts) != 2 {
					c.logger.Warn(fmt.Sprintf("metric pair '%s' not in key=value format", pair))
					continue
				}
				info[parts[0]] = parts[1]

			}

		}
	}
	info["name"] = namespace
	return info, nil
}

func (c *defaultASClient) Info() (map[string]string, error) {
	if err := c.conn.SetTimeout(time.Now().Add(c.policy.Timeout), c.policy.Timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}

	response, err := c.conn.RequestInfo(defaultNodeInfoCommands...)

	// Try to login and get a new session
	if err != nil && err.Matches(types.EXPIRED_SESSION) {
		if loginErr := c.conn.Login(c.policy); loginErr != nil {
			return nil, loginErr
		}
	}

	if err != nil {
		return nil, err
	}
	info := make(map[string]string)
	for k, v := range response {
		switch k {
		case "statistics":
			for _, pair := range splitFields(v) {
				parts := splitPair(pair)
				if len(parts) != 2 {
					c.logger.Warn(fmt.Sprintf("metric pair '%s' not in key=value format", pair))
					continue
				}
				info[parts[0]] = parts[1]

			}
		default:
			info[k] = v
		}
	}
	return info, nil
}

func (c *defaultASClient) Close() {
	c.conn.Close()
}

// splitPair splits a metric pair in the format of 'key=value'
func splitPair(pair string) []string {
	return strings.Split(pair, "=")
}

// splitFields splits a string of metric pairs delimited with the ';' character
func splitFields(info string) []string {
	return strings.Split(info, ";")
}
