// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestServerAddressAndPort(t *testing.T) {
	tests := []struct {
		name            string
		serverStatus    bson.M
		expectedAddress string
		expectedPort    int64
		expectedErr     error
	}{
		{
			name:            "address_only",
			serverStatus:    bson.M{"host": "localhost"},
			expectedAddress: "localhost",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name:            "address_and_port",
			serverStatus:    bson.M{"host": "localhost:27018"},
			expectedAddress: "localhost",
			expectedPort:    27018,
		},
		{
			name:            "ipv4",
			serverStatus:    bson.M{"host": "10.0.0.7:27017"},
			expectedAddress: "10.0.0.7",
			expectedPort:    27017,
		},
		{
			name:            "bare_ipv6_loopback",
			serverStatus:    bson.M{"host": "::1"},
			expectedAddress: "::1",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name:            "bracketed_ipv6_loopback_without_port",
			serverStatus:    bson.M{"host": "[::1]"},
			expectedAddress: "::1",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name:            "bracketed_ipv6_loopback_with_port",
			serverStatus:    bson.M{"host": "[::1]:27018"},
			expectedAddress: "::1",
			expectedPort:    27018,
		},
		{
			name:            "bare_ipv6",
			serverStatus:    bson.M{"host": "2001:db8::1"},
			expectedAddress: "2001:db8::1",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name:            "bracketed_ipv6_with_port",
			serverStatus:    bson.M{"host": "[2001:db8::1]:27017"},
			expectedAddress: "2001:db8::1",
			expectedPort:    27017,
		},
		{
			// MongoDB appends a non-default port to the host name without bracketing it, so an
			// IPv6 host name on a non-default port arrives in this shape.
			name:            "bare_ipv6_loopback_with_port",
			serverStatus:    bson.M{"host": "::1:27018"},
			expectedAddress: "::1",
			expectedPort:    27018,
		},
		{
			name:            "bare_ipv6_with_port",
			serverStatus:    bson.M{"host": "2001:db8::1:27018"},
			expectedAddress: "2001:db8::1",
			expectedPort:    27018,
		},
		{
			// Unbracketed, "::1:2701" is both a valid address and a plausible host and port. The
			// address reading wins, because splitting is only attempted once parsing has failed.
			name:            "bare_ipv6_that_is_itself_an_address",
			serverStatus:    bson.M{"host": "::1:2701"},
			expectedAddress: "::1:2701",
			expectedPort:    defaultMongoDBPort,
		},
		{
			name:         "missing_host",
			serverStatus: bson.M{},
			expectedErr:  errors.New("host field not found in server status"),
		},
		{
			name:         "invalid_port",
			serverStatus: bson.M{"host": "localhost:invalid"},
			expectedErr:  errors.New("failed to parse port: strconv.ParseInt: parsing \"invalid\": invalid syntax"),
		},
		{
			name:         "invalid_host_format",
			serverStatus: bson.M{"host": "localhost:27018:extra"},
			expectedErr:  errors.New("unexpected host format: localhost:27018:extra"),
		},
		{
			name:         "ipv6_with_out_of_range_port",
			serverStatus: bson.M{"host": "::1:99999"},
			expectedErr:  errors.New("unexpected host format: ::1:99999"),
		},
		{
			name:         "ipv6_with_non_numeric_trailing_segment",
			serverStatus: bson.M{"host": "::1:lastbit"},
			expectedErr:  errors.New("unexpected host format: ::1:lastbit"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, port, err := serverAddressAndPort(tt.serverStatus)
			if tt.expectedErr != nil {
				require.EqualError(t, err, tt.expectedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedAddress, address)
			require.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestResolveLoopbackHost(t *testing.T) {
	const collectorHostName = "collector-host"

	tests := []struct {
		name              string
		host              string
		collectorHostName string
		expected          string
	}{
		{name: "localhost", host: "localhost", collectorHostName: collectorHostName, expected: collectorHostName},
		{name: "localhost is matched case insensitively", host: "Localhost", collectorHostName: collectorHostName, expected: collectorHostName},
		{name: "IPv4 loopback", host: "127.0.0.1", collectorHostName: collectorHostName, expected: collectorHostName},
		{name: "IPv4 loopback outside 127.0.0.1", host: "127.1.2.3", collectorHostName: collectorHostName, expected: collectorHostName},
		{name: "IPv6 loopback", host: "::1", collectorHostName: collectorHostName, expected: collectorHostName},
		{name: "remote host", host: "mongohost", collectorHostName: collectorHostName, expected: "mongohost"},
		{name: "remote IPv4", host: "10.0.0.7", collectorHostName: collectorHostName, expected: "10.0.0.7"},
		{name: "remote IPv6", host: "2001:db8::1", collectorHostName: collectorHostName, expected: "2001:db8::1"},
		// A container host name is not loopback knowledge, so it is reported as the node gave it.
		{name: "container host name", host: "24f7cb73cc66", collectorHostName: collectorHostName, expected: "24f7cb73cc66"},
		// Without a collector host name there is nothing better to report than what the node said.
		{name: "unknown collector host name", host: "localhost", collectorHostName: "", expected: "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, resolveLoopbackHost(tt.host, tt.collectorHostName))
		})
	}
}

func TestResolveCollectorHostName(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	require.Equal(t, hostname, resolveCollectorHostName(receivertest.NewNopSettings(metadata.Type).Logger))
}

func TestResolveServerAddress(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
	require.Equal(t, hostname, s.collectorHostName)

	t.Run("loopback reports the collector host name", func(t *testing.T) {
		address, port, err := s.resolveServerAddress(bson.M{"host": "127.0.0.1:27017"})
		require.NoError(t, err)
		require.Equal(t, hostname, address)
		require.Equal(t, int64(27017), port)
	})

	t.Run("unbracketed IPv6 loopback with a port reports the collector host name", func(t *testing.T) {
		address, port, err := s.resolveServerAddress(bson.M{"host": "::1:27018"})
		require.NoError(t, err)
		require.Equal(t, hostname, address)
		require.Equal(t, int64(27018), port)
	})

	t.Run("remote host is reported verbatim", func(t *testing.T) {
		address, port, err := s.resolveServerAddress(bson.M{"host": "mongohost:27018"})
		require.NoError(t, err)
		require.Equal(t, "mongohost", address)
		require.Equal(t, int64(27018), port)
	})

	t.Run("unresolvable collector host name reports the loopback address", func(t *testing.T) {
		unknown := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
		unknown.collectorHostName = ""

		address, port, err := unknown.resolveServerAddress(bson.M{"host": "localhost:27017"})
		require.NoError(t, err)
		require.Equal(t, "localhost", address)
		require.Equal(t, int64(27017), port)
	})

	t.Run("parse failure is surfaced", func(t *testing.T) {
		_, _, err := s.resolveServerAddress(bson.M{})
		require.EqualError(t, err, "host field not found in server status")
	})
}

// TestResolveServerAddressSeedsServiceInstanceID guards that server.address and service.instance.id
// are taken from the same resolution, so the two can never name different machines.
func TestResolveServerAddressSeedsServiceInstanceID(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))

	address, port, err := s.resolveServerAddress(bson.M{"host": "localhost:27017"})
	require.NoError(t, err)
	require.Equal(t, generateInstanceID(hostname, port), generateInstanceID(address, port))
	require.NotEqual(t, generateInstanceID("localhost", port), generateInstanceID(address, port))
}

// TestServiceInstanceIDUnchangedForNonLoopback pins the identifiers published before loopback
// resolution existed, so no non-loopback deployment sees its service.instance.id move.
func TestServiceInstanceIDUnchangedForNonLoopback(t *testing.T) {
	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))

	tests := []struct {
		host     string
		expected string
	}{
		{host: "mongodb-host-1", expected: "8ab7c64b-866c-5cae-aa55-67d463988c7f"},
		{host: "ecb6adf34046", expected: "0d53bbba-e245-5e46-a385-b3cb9baa6d28"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			address, port, err := s.resolveServerAddress(bson.M{"host": tt.host})
			require.NoError(t, err)
			require.Equal(t, tt.host, address)
			require.Equal(t, tt.expected, generateInstanceID(address, port))
		})
	}
}
