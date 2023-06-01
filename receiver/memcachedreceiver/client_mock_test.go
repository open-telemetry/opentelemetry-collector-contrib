// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
