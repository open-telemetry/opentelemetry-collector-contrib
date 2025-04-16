// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"crypto/tls"
	"strings"
	"sync"
	"time"

	as "github.com/aerospike/aerospike-client-go/v7"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/cluster"
)

var defaultNodeInfoCommands = []string{
	"node",
	"statistics",
}

// nodeName: metricName: stats
type clusterInfo = map[string]map[string]string

// Aerospike is the interface that provides information about a given node
type Aerospike interface {
	// NamespaceInfo gets information about a specific namespace
	NamespaceInfo() namespaceInfo
	// Info gets high-level information about the node/system.
	Info() clusterInfo
	// Close closes the connection to the Aerospike node
	Close()
}

type clientConfig struct {
	host                  *as.Host
	username              string
	password              string
	timeout               time.Duration
	logger                *zap.SugaredLogger
	collectClusterMetrics bool
	tls                   *tls.Config
}

type nodeGetter interface {
	GetNodes() []cluster.Node
	Close()
}

type defaultASClient struct {
	cluster nodeGetter
	policy  *as.ClientPolicy   // Timeout and authentication information
	logger  *zap.SugaredLogger // logs malformed metrics in responses
}

type nodeGetterFactoryFunc func(cfg *clientConfig, policy *as.ClientPolicy, authEnabled bool) (nodeGetter, error)

func nodeGetterFactory(cfg *clientConfig, policy *as.ClientPolicy, authEnabled bool) (nodeGetter, error) {
	hosts := []*as.Host{cfg.host}

	if cfg.collectClusterMetrics {
		cluster, err := cluster.NewCluster(policy, hosts)
		return cluster, err
	}

	cluster, err := cluster.NewSubsetCluster(
		policy,
		hosts,
		authEnabled,
	)
	return cluster, err
}

// newASClient creates a new defaultASClient connected to the given host and port
// If collectClusterMetrics is true, the client will connect to and tend all nodes in the cluster
// If username and password aren't blank, they're used to authenticate
func newASClient(cfg *clientConfig, ngf nodeGetterFactoryFunc) (*defaultASClient, error) {
	authEnabled := cfg.username != "" && cfg.password != ""

	policy := as.NewClientPolicy()
	policy.Timeout = cfg.timeout
	if authEnabled {
		policy.User = cfg.username
		policy.Password = cfg.password
	}

	if cfg.tls != nil {
		// enable TLS
		policy.TlsConfig = cfg.tls
	}

	cluster, err := ngf(cfg, policy, authEnabled)
	if err != nil {
		return nil, err
	}

	return &defaultASClient{
		cluster: cluster,
		logger:  cfg.logger,
		policy:  policy,
	}, nil
}

// useNodeFunc maps a nodeFunc to all the client's nodes
func (c *defaultASClient) useNodeFunc(nf nodeFunc) clusterInfo {
	var res clusterInfo

	nodes := c.cluster.GetNodes()

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

// metricName: stat
// may be used as, commandName: stat
type metricsMap = map[string]string

// Info returns a clusterInfo map of node names to metricMaps
// it uses the info commands defined in defaultNodeInfoCommands
func (c *defaultASClient) Info() clusterInfo {
	res := clusterInfo{}

	// NOTE this discards the command names
	metricsToParse := c.useNodeFunc(allNodeInfo)
	for node, commands := range metricsToParse {
		res[node] = metricsMap{}
		for command, stats := range commands {
			ps := parseStats(command, stats, ";")
			mergeMetricsMap(res[node], ps, c.logger)
		}
	}

	return res
}

// nodeName: namespaceName: metricName: stats
type namespaceInfo = map[string]map[string]map[string]string

// NamespaceInfo returns a namespaceInfo map
// the map contains the results of the "namespace/<name>" info command
// for all nodes' namespaces
func (c *defaultASClient) NamespaceInfo() namespaceInfo {
	res := namespaceInfo{}

	metricsToParse := c.useNodeFunc(allNamespaceInfo)
	for node, namespaces := range metricsToParse {
		res[node] = map[string]map[string]string{}
		for ns, stats := range namespaces {
			// ns == "namespace/<namespaceName>"
			nsData := strings.SplitN(ns, "/", 2)
			if len(nsData) < 2 {
				c.logger.Warn("NamespaceInfo nsData len < 2")
				continue
			}
			nsName := nsData[1]
			res[node][nsName] = parseStats(ns, stats, ";")
		}
	}

	return res
}

// Close closes the client's connections to all nodes
func (c *defaultASClient) Close() {
	c.cluster.Close()
}

// mapNodeInfoFunc maps a nodeFunc to all nodes in the list in parallel
// if an error occurs during any of the nodeFuncs' execution, it is logged but not returned
// the return value is a clusterInfo map from node name to command to unparsed metric string
func mapNodeInfoFunc(nodes []cluster.Node, nodeF nodeFunc, policy *as.InfoPolicy, logger *zap.SugaredLogger) clusterInfo {
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
				logger.Errorf("mapNodeInfoFunc err: %v", err)
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

// nodeFunc is a function that requests info commands from a node
// and returns a metricsMap from command to metric string
type nodeFunc func(n cluster.Node, policy *as.InfoPolicy) (metricsMap, error)

// namespaceNames is used by nodeFuncs to get the names of all namespaces ona node
func namespaceNames(n cluster.Node, policy *as.InfoPolicy) ([]string, error) {
	var namespaces []string

	info, err := n.RequestInfo(policy, "namespaces")
	if err != nil {
		return nil, err
	}

	namespaces = strings.Split(info["namespaces"], ";")
	return namespaces, err
}

// allNodeInfo returns the results of defaultNodeInfoCommands for a node
func allNodeInfo(n cluster.Node, policy *as.InfoPolicy) (metricsMap, error) {
	var res metricsMap
	commands := defaultNodeInfoCommands

	res, err := n.RequestInfo(policy, commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// allNamespaceInfo returns the results of namespace/%s for each namespace on the node
func allNamespaceInfo(n cluster.Node, policy *as.InfoPolicy) (metricsMap, error) {
	var res metricsMap

	names, err := namespaceNames(n, policy)
	if err != nil {
		return nil, err
	}

	commands := make([]string, len(names))
	for i, name := range names {
		commands[i] = "namespace/" + name
	}

	res, err = n.RequestInfo(policy, commands...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func parseStats(defaultKey, s, sep string) metricsMap {
	stats := make(metricsMap, strings.Count(s, sep)+1)
	s2 := strings.Split(s, sep)
	for _, s := range s2 {
		list := strings.SplitN(s, "=", 2)
		switch len(list) {
		case 0:
		case 1:
			stats[defaultKey] = list[0]
		case 2:
			stats[list[0]] = list[1]
		default:
			stats[list[0]] = strings.Join(list[1:], "=")
		}
	}

	return stats
}

// mergeMetricsMap merges values from rm into lm
// logs a warning if a duplicate key is found
func mergeMetricsMap(lm, rm metricsMap, logger *zap.SugaredLogger) {
	for k, v := range rm {
		if _, ok := lm[k]; ok {
			logger.Warnf("duplicate key: %s", k)
		}
		lm[k] = v
	}
}
