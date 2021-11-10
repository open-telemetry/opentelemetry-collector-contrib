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
