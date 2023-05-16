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
	str, err := p.client.retrieveInfo()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(str, p.delimiter)
	attrs := make(map[string]string)
	for _, line := range lines {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) == 2 { // defensive, should always == 2
			attrs[pair[0]] = pair[1]
		}
	}
	return attrs, nil
}
