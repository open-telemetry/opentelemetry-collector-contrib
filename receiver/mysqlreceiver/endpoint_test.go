// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
)

func endpointConfig(endpoint string, transport confignet.TransportType) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.AddrConfig = confignet.AddrConfig{Endpoint: endpoint, Transport: transport}
	return cfg
}

func TestResolveServerEndpoint(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	tests := []struct {
		name      string
		endpoint  string
		transport confignet.TransportType
		expected  resolvedEndpoint
	}{
		{
			name:      "loopback by name resolves to the collector host",
			endpoint:  "localhost:3306",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        hostname,
				port:           3306,
				hasPort:        true,
				instanceIDSeed: net.JoinHostPort(hostname, "3306"),
			},
		},
		{
			name:      "IPv4 loopback resolves to the collector host",
			endpoint:  "127.0.0.1:3306",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        hostname,
				port:           3306,
				hasPort:        true,
				instanceIDSeed: net.JoinHostPort(hostname, "3306"),
			},
		},
		{
			name:      "IPv6 loopback resolves to the collector host",
			endpoint:  "[::1]:3306",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        hostname,
				port:           3306,
				hasPort:        true,
				instanceIDSeed: net.JoinHostPort(hostname, "3306"),
			},
		},
		{
			name:      "loopback on a non-default port keeps the port",
			endpoint:  "localhost:3307",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        hostname,
				port:           3307,
				hasPort:        true,
				instanceIDSeed: net.JoinHostPort(hostname, "3307"),
			},
		},
		{
			name:      "remote host is reported verbatim",
			endpoint:  "db.example.com:3306",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        "db.example.com",
				port:           3306,
				hasPort:        true,
				instanceIDSeed: "db.example.com:3306",
			},
		},
		{
			name:      "remote IP is reported verbatim",
			endpoint:  "10.1.2.3:3306",
			transport: confignet.TransportTypeTCP,
			expected: resolvedEndpoint{
				address:        "10.1.2.3",
				port:           3306,
				hasPort:        true,
				instanceIDSeed: "10.1.2.3:3306",
			},
		},
		{
			name:      "unix socket reports the path and no port",
			endpoint:  "/var/run/mysqld/mysqld.sock",
			transport: confignet.TransportTypeUnix,
			expected: resolvedEndpoint{
				address:        "/var/run/mysqld/mysqld.sock",
				instanceIDSeed: "/var/run/mysqld/mysqld.sock",
			},
		},
		{
			name:      "endpoint without a port reports neither attribute",
			endpoint:  "localhost",
			transport: confignet.TransportTypeTCP,
			expected:  resolvedEndpoint{instanceIDSeed: "localhost"},
		},
		{
			name:      "unparsable port reports neither attribute",
			endpoint:  "db.example.com:mysql",
			transport: confignet.TransportTypeTCP,
			expected:  resolvedEndpoint{instanceIDSeed: "db.example.com:mysql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveServerEndpoint(endpointConfig(tt.endpoint, tt.transport), zap.NewNop()))
		})
	}
}

// TestServerAddressMatchesInstanceIDSeed guards against the two values drifting apart: a resource
// must not claim a server.address on one machine while its service.instance.id identifies another.
func TestServerAddressMatchesInstanceIDSeed(t *testing.T) {
	for _, endpoint := range []string{"localhost:3306", "127.0.0.1:3306", "[::1]:3306", "db.example.com:3306"} {
		t.Run(endpoint, func(t *testing.T) {
			resolved := resolveServerEndpoint(endpointConfig(endpoint, confignet.TransportTypeTCP), zap.NewNop())

			seedHost, _, err := net.SplitHostPort(resolved.instanceIDSeed)
			require.NoError(t, err)
			assert.Equal(t, resolved.address, seedHost)
		})
	}
}

// TestInstanceIDSeedUnchangedForNonLoopback pins the seed to the raw endpoint wherever no loopback
// rewrite applies, so existing service.instance.id UUIDs do not change.
func TestInstanceIDSeedUnchangedForNonLoopback(t *testing.T) {
	tests := []struct {
		endpoint  string
		transport confignet.TransportType
	}{
		{"db.example.com:3306", confignet.TransportTypeTCP},
		{"10.1.2.3:3306", confignet.TransportTypeTCP},
		{"/var/run/mysqld/mysqld.sock", confignet.TransportTypeUnix},
		{"localhost", confignet.TransportTypeTCP},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			resolved := resolveServerEndpoint(endpointConfig(tt.endpoint, tt.transport), zap.NewNop())
			assert.Equal(t, tt.endpoint, resolved.instanceIDSeed)
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "127.0.0.53", "::1"} {
		assert.True(t, isLoopbackHost(host), host)
	}
	for _, host := range []string{"db.example.com", "10.1.2.3", "0.0.0.0", ""} {
		assert.False(t, isLoopbackHost(host), host)
	}
}
