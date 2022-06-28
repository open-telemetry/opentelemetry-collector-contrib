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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"
)

var defaultNodeInfoCommands = []string{
	"namespaces",
	"node",
	"statistics",
	"services",
}

type clusterInfo map[string]map[string]string // TODO what if this is connected to multiple clusters?
type metricsMap map[string]string

// aerospike is the interface that provides information about a given node
type aerospike interface {
	// NamespaceInfo gets information about a specific namespace
	NamespaceInfo(namespace string) clusterInfo
	// Info gets high-level information about the node/system.
	Info() clusterInfo
	// Close closes the connection to the Aerospike node
	Close()
}

type clientConfig struct {
	host                  string
	port                  int
	username              string
	password              string
	timeout               time.Duration
	logger                *zap.Logger
	collectClusterMetrics bool
}

type nodeGettor interface {
	GetNodes() []cluster.Node
	Close()
}

type defaultASClient struct {
	cluster nodeGettor
	policy  *as.ClientPolicy // Timeout and authentication information
	logger  *zap.Logger      // logs malformed metrics in responses
}

func nodeGettorFactory(cfg *clientConfig, policy *as.ClientPolicy) (nodeGettor, error) {
	authEnabled := cfg.username != "" && cfg.password != ""

	hosts := []*as.Host{as.NewHost(cfg.host, cfg.port)}

	if cfg.collectClusterMetrics {
		cluster, err := cluster.NewSubsetCluster(
			policy,
			hosts,
			authEnabled,
		)
		return cluster, err
	}

	cluster, err := cluster.NewCluster(policy, hosts)
	return cluster, err

}

// newASClient creates a new defaultASClient connected to the given host and port.
// If username and password aren't blank, they're used to authenticate
func newASClient(cfg *clientConfig) (*defaultASClient, error) {
	authEnabled := cfg.username != "" && cfg.password != ""

	policy := as.NewClientPolicy()
	policy.Timeout = cfg.timeout
	if authEnabled {
		policy.User = cfg.username
		policy.Password = cfg.password
	}

	cluster, err := nodeGettorFactory(cfg, policy)
	if err != nil {
		return nil, err
	}

	return &defaultASClient{
		cluster: cluster,
		logger:  cfg.logger,
		policy:  policy,
	}, nil
}

func (c *defaultASClient) useInfoFunc(nf nodeFunc) clusterInfo {
	var nodes []cluster.Node
	var res clusterInfo

	nodes = c.cluster.GetNodes()

	policy := as.NewInfoPolicy()
	policy.Timeout = c.policy.Timeout

	res = mapNodeInfoFunc(
		nodes,
		nf,
		policy,
		c.logger,
	)

	return res
}

func (c *defaultASClient) Info() clusterInfo {
	return c.useInfoFunc(allNodeInfo)
}

func (c *defaultASClient) NamespaceInfo() clusterInfo {
	return c.useInfoFunc(allNamespaceInfo)
}

func (c *defaultASClient) Close() {
	c.cluster.Close()
}

func mapNodeInfoFunc(nodes []cluster.Node, nodeF nodeFunc, policy *as.InfoPolicy, logger *zap.Logger) clusterInfo {
	numNodes := len(nodes)
	res := make(clusterInfo, numNodes)

	type nodeStats struct {
		name    string
		metrics metricsMap
		error   error
	}

	var wg sync.WaitGroup
	resChan := make(chan nodeStats, numNodes)

	for _, nd := range nodes {
		wg.Add(1)
		go func(nd cluster.Node) {
			defer wg.Done()

			name := nd.GetName()
			metrics, err := nodeF(nd, policy)
			if err != nil {
				logger.Sugar().Errorf("mapNodeInfoFunc err: %s", err)
			}

			ns := nodeStats{
				name:    name,
				metrics: metrics,
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

// node wise info functions

type nodeFunc func(n cluster.Node, policy *as.InfoPolicy) (map[string]string, error)

func namespaceNames(n cluster.Node, policy *as.InfoPolicy) ([]string, error) {
	var namespaces []string

	info, err := n.RequestInfo(policy, "namespaces")
	if err != nil {
		return nil, err // TODO maybe add something via partial error
	}

	namespaces = strings.Split(info["namespaces"], ";")
	return namespaces, err
}

func allNodeInfo(n cluster.Node, policy *as.InfoPolicy) (map[string]string, error) {
	var res map[string]string
	commands := defaultNodeInfoCommands

	res, err := n.RequestInfo(policy, commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func allNamespaceInfo(n cluster.Node, policy *as.InfoPolicy) (map[string]string, error) {
	var res map[string]string

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
