package memcachedreceiver

import (
	"net"
	"time"

	"github.com/grobie/gomemcache/memcache"
)

type client interface {
	SetTimeout(time.Duration)
	Init(string) error
	Stats() (map[net.Addr]memcache.Stats, error)
}

type memcachedClient struct {
	client *memcache.Client
}

var _ client = (*memcachedClient)(nil)

func (c *memcachedClient) Init(endpoint string) error {
	newClient, err := memcache.New(endpoint)
	if err != nil {
		return err
	}
	c.client = newClient
	return nil
}

func (c *memcachedClient) SetTimeout(t time.Duration) {
	c.client.Timeout = t
}

func (c *memcachedClient) Stats() (map[net.Addr]memcache.Stats, error) {
	return c.client.Stats()
}
