// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memcachedreceiver

import (
	"encoding/json"
	"net"
	"os"

	"github.com/grobie/gomemcache/memcache"
)

type fakeClient struct{}

var _ client = (*fakeClient)(nil)

func (c *fakeClient) Stats() (map[net.Addr]memcache.Stats, error) {
	bytes, err := os.ReadFile("./testdata/fake_stats.json")
	if err != nil {
		return nil, err
	}
	stats := make(map[net.Addr]memcache.Stats)
	statsJSON := make(map[string]memcache.Stats)
	err = json.Unmarshal(bytes, &statsJSON)
	if err != nil {
		return nil, err
	}

	for addr, s := range statsJSON {
		tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}
		stats[tcpaddr] = s
	}

	return stats, nil
}
