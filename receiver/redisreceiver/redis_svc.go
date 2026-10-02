// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"strings"

	"go.uber.org/zap"
)

// Wraps a client, parses the Redis info command, returning a string-string map
// containing all of the key value pairs returned by INFO. Takes a line delimiter
// from the passed in client to support testing, because Redis uses CRLF and test
// data uses LF.
type redisSvc struct {
	client    client
	delimiter string
	logger    *zap.Logger
}

// Creates a new redisSvc. Pass in a client implementation.
func newRedisSvc(client client, logger *zap.Logger) *redisSvc {
	return &redisSvc{
		client:    client,
		delimiter: client.delimiter(),
		logger:    logger,
	}
}

// Calls the Redis INFO command, and, if the server has cluster mode enabled, also CLUSTER INFO,
// returning a merged `info` map. CLUSTER INFO errors on a standalone (non-cluster) server, so it's
// only attempted when INFO's own cluster_enabled field says the server is actually a cluster node.
// Even then, it's fetched best-effort: if it fails, the metrics derived from INFO are still
// returned rather than failing the whole scrape.
func (p *redisSvc) info() (info, error) {
	str, err := p.client.retrieveInfo()
	if err != nil {
		return nil, err
	}
	attrs := p.parseAttrs(str)

	if attrs["cluster_enabled"] != "1" {
		return attrs, nil
	}

	if clusterStr, clusterErr := p.client.retrieveClusterInfo(); clusterErr == nil {
		for k, v := range p.parseAttrs(clusterStr) {
			attrs[k] = v
		}
	} else {
		p.logger.Warn("failed to retrieve CLUSTER INFO; redis.cluster.* metrics will be unavailable for this scrape",
			zap.Error(clusterErr))
	}

	return attrs, nil
}

// parseAttrs turns delimited "key:value" lines, as returned by INFO and CLUSTER INFO,
// into a string-string map.
func (p *redisSvc) parseAttrs(str string) map[string]string {
	lines := strings.Split(str, p.delimiter)
	attrs := make(map[string]string)
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) == 2 { // defensive, should always == 2
			attrs[pair[0]] = pair[1]
		}
	}
	return attrs
}
