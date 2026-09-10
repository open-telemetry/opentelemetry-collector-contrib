// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name               string
		cfg                *Config
		expectedUniqueErrs []error
		expectedCount      int
	}{
		{
			name:               "no dns servers or hostnames",
			cfg:                &Config{},
			expectedUniqueErrs: []error{errMissingDNSServers, errMissingHostnames},
			expectedCount:      2,
		},
		{
			name:               "missing hostnames",
			cfg:                &Config{DNSServers: []DNSServerConfig{{Endpoint: "8.8.8.8"}}},
			expectedUniqueErrs: []error{errMissingHostnames},
			expectedCount:      1,
		},
		{
			name:               "missing dns servers",
			cfg:                &Config{Hostnames: []HostnameConfig{{Name: "example.com"}}},
			expectedUniqueErrs: []error{errMissingDNSServers},
			expectedCount:      1,
		},
		{
			name: "empty dns server endpoint",
			cfg: &Config{
				DNSServers: []DNSServerConfig{{Endpoint: ""}},
				Hostnames:  []HostnameConfig{{Name: "example.com"}},
			},
			expectedUniqueErrs: []error{errMissingDNSServer},
			expectedCount:      1,
		},
		{
			name: "invalid dns server network",
			cfg: &Config{
				DNSServers: []DNSServerConfig{{Endpoint: "8.8.8.8", Network: "quic"}},
				Hostnames:  []HostnameConfig{{Name: "example.com"}},
			},
			expectedUniqueErrs: []error{errInvalidDNSServerNetwork},
			expectedCount:      1,
		},
		{
			name: "empty hostname name",
			cfg: &Config{
				DNSServers: []DNSServerConfig{{Endpoint: "8.8.8.8"}},
				Hostnames:  []HostnameConfig{{Name: ""}},
			},
			expectedUniqueErrs: []error{errMissingHostname},
			expectedCount:      1,
		},
		{
			name: "multiple empty hostname names",
			cfg: &Config{
				DNSServers: []DNSServerConfig{{Endpoint: "8.8.8.8"}},
				Hostnames:  []HostnameConfig{{Name: ""}, {Name: ""}},
			},
			expectedUniqueErrs: []error{errMissingHostname},
			expectedCount:      2,
		},
		{
			name: "all valid",
			cfg: &Config{
				DNSServers: []DNSServerConfig{
					{Endpoint: "8.8.8.8"},
					{Endpoint: "1.1.1.1:53", Network: "udp", Timeout: 5 * time.Second},
					{Endpoint: "9.9.9.9:853", Network: "tcp-tls", Timeout: 10 * time.Second},
				},
				Hostnames: []HostnameConfig{{Name: "example.com", RecordType: "A"}},
			},
			expectedUniqueErrs: []error{},
			expectedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedCount == 0 {
				require.NoError(t, err, "expected no error")
				return
			}
			require.Error(t, err, "expected error(s)")

			all := multierr.Errors(err)
			require.Len(t, all, tt.expectedCount, "unexpected number of collected errors: %v", all)

			for _, expected := range tt.expectedUniqueErrs {
				require.ErrorIs(t, err, expected, "expected error not found")
			}

			for _, got := range all {
				found := false
				for _, expected := range tt.expectedUniqueErrs {
					if errors.Is(got, expected) {
						found = true
						break
					}
				}
				require.True(t, found, "unexpected error returned: %v", got)
			}
		})
	}
}
