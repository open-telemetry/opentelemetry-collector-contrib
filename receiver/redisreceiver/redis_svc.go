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

// Calls the Redis INFO command on the client and returns an `info` map.
func (p *redisSvc) info() (info, error) {
	strInfo, err := p.client.retrieveInfo()
	if err != nil {
		return nil, err
	}

	strClusterInfo, err := p.client.retrieveClusterInfo()
	if err != nil {
		return nil, err
	}

	attrs := make(map[string]string)
	for _, str := range [2]string{strInfo, strClusterInfo} {
		lines := strings.Split(str, p.delimiter)
		for _, line := range lines {
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}
			pair := strings.Split(line, ":")
			if len(pair) == 2 { // defensive, should always == 2
				attrs[pair[0]] = pair[1]
			}
		}
	}
	return attrs, nil
}
