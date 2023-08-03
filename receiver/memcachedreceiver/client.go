// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memcachedreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver"

import (
	"net"
	"time"

	"github.com/grobie/gomemcache/memcache"
)

type client interface {
	Stats() (map[net.Addr]memcache.Stats, error)
}

type newMemcachedClientFunc func(endpoint string, timeout time.Duration) (client, error)

func newMemcachedClient(endpoint string, timeout time.Duration) (client, error) {
	newClient, err := memcache.New(endpoint)
	if err != nil {
		return nil, err
	}

	newClient.Timeout = timeout
	return newClient, nil
}
