// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import "strings"

// Wraps a client, parses the Redis info command, returning a string-string map
// containing all of the key value pairs returned by INFO. Takes a line delimiter
// from the passed in client to support testing, because Redis uses CRLF and test
// data uses LF.
type redisSvc struct {
	client    client
	delimiter string
}

// Creates a new redisSvc. Pass in a client implementation.
func newRedisSvc(client client) *redisSvc {
	return &redisSvc{
		client:    client,
		delimiter: client.delimiter(),
	}
}

// Calls the Redis INFO and CLUSTER INFO commands on the client and returns a merged `info` map.
// CLUSTER INFO is fetched best-effort: if it fails (e.g. the command is restricted), the
// metrics derived from INFO are still returned rather than failing the whole scrape.
func (p *redisSvc) info() (info, error) {
	str, err := p.client.retrieveInfo()
	if err != nil {
		return nil, err
	}
	attrs := p.parseAttrs(str)

	if clusterStr, clusterErr := p.client.retrieveClusterInfo(); clusterErr == nil {
		for k, v := range p.parseAttrs(clusterStr) {
			attrs[k] = v
		}
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
