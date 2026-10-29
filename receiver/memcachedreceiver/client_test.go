// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memcachedreceiver

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/grobie/gomemcache/memcache"
	"github.com/stretchr/testify/require"
)

func TestNewMemcachedClientTLS(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig *tls.Config
	}{
		{
			name:      "plaintext when tls config is nil",
			tlsConfig: nil,
		},
		{
			name:      "tls config is propagated to the client",
			tlsConfig: &tls.Config{ServerName: "memcached.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := newMemcachedClient("localhost:11211", 10*time.Second, tt.tlsConfig)
			require.NoError(t, err)

			mc, ok := c.(*memcache.Client)
			require.True(t, ok)
			if tt.tlsConfig == nil {
				require.Nil(t, mc.TlsConfig)
			} else {
				require.Same(t, tt.tlsConfig, mc.TlsConfig)
			}
			require.Equal(t, 10*time.Second, mc.Timeout)
		})
	}
}
