package memcachedreceiver

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"time"

	"github.com/grobie/gomemcache/memcache"
)

type fakeClient struct{}

var _ client = (*fakeClient)(nil)

func (c *fakeClient) Init(endpoint string) error {
	return nil
}

func (c *fakeClient) SetTimeout(t time.Duration) {}

func (c *fakeClient) Stats() (map[net.Addr]memcache.Stats, error) {
	bytes, err := ioutil.ReadFile("./testdata/fake_stats.json")
	if err != nil {
		return nil, err
	}
	stats := make(map[net.Addr]memcache.Stats)
	statsJson := make(map[string]memcache.Stats)
	err = json.Unmarshal(bytes, &statsJson)
	if err != nil {
		return nil, err
	}

	for addr, s := range statsJson {
		tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}
		stats[tcpaddr] = s
	}

	return stats, nil
}
