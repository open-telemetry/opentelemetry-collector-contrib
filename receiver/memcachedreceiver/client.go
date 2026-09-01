// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memcachedreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver"

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/grobie/gomemcache/memcache"
)

type client interface {
	Stats() (map[net.Addr]memcache.Stats, error)
}

type newMemcachedClientFunc func(endpoint string, timeout time.Duration, tlsConfig *tls.Config) (client, error)

func newMemcachedClient(endpoint string, timeout time.Duration, tlsConfig *tls.Config) (client, error) {
	newClient, err := memcache.New(endpoint)
	if err != nil {
		return nil, err
	}

	newClient.Timeout = timeout
	// A nil tlsConfig leaves the connection as plaintext (the default).
	newClient.TlsConfig = tlsConfig
	return newClient, nil
}
