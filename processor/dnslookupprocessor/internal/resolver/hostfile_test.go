// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"
)

// valid hosts file with uppercase and lowercase
const validHostFileContent = `
127.0.0.1 localhost
192.168.1.10 Example.com test.local
192.168.1.11 example2.com
::1 ip6-localhost ip6-loopback
`

// valid hosts file
const validHostFileContent2 = `
192.168.1.10 example.com example2.com
192.168.1.20 another.example.com
192.168.1.30 test.example.com
`

// Hosts file with comments and empty lines
const hostFileWithCommentsAndEmptyLines = `
# This is a comment
127.0.0.1 localhost

# No hostname
192.168.1.66

# Another comment
192.168.1.20 commented.example.com # Inline comment

  # Indented comment

`

// Hosts file with some invalid entries
const hostFileWithInvalidEntries = `
invalid-ip hostname.example.com
192.168.1.30 valid.example.com
192.168.1.40 invalid$hostname.com
no-ip-or-hostname
`

func TestNewHostFileResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name          string
		hostFilePaths []string
		expectError   bool
		expectedError error
	}{
		{
			name:          "Valid hostfile",
			hostFilePaths: []string{testutil.CreateTempHostFile(t, validHostFileContent)},
			expectError:   false,
		},
		{
			name:          "Multiple valid hostfiles",
			hostFilePaths: []string{testutil.CreateTempHostFile(t, validHostFileContent), testutil.CreateTempHostFile(t, validHostFileContent2)},
			expectError:   false,
		},
		{
			name:          "Empty hostfile paths",
			hostFilePaths: []string{},
			expectError:   true,
			expectedError: ErrInvalidHostFilePath,
		},
		{
			name:          "Non-existent hostfile",
			hostFilePaths: []string{"/non/existent/path"},
			expectError:   true,
			expectedError: ErrInvalidHostFilePath,
		},
		{
			name:          "One valid, one invalid hostfile",
			hostFilePaths: []string{testutil.CreateTempHostFile(t, validHostFileContent), "/non/existent/path"},
			expectError:   true,
			expectedError: ErrInvalidHostFilePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewHostFileResolver(tt.hostFilePaths, logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resolver)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resolver)
				assert.NotEmpty(t, resolver.hostnameToIP)
				assert.NotEmpty(t, resolver.ipToHostname)
			}
		})
	}
}

func TestHostFileResolver_LoadAndParseHostFiles(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name                string
		hostFileContent     []string
		expectedHostnameIPs map[string][]string
		expectedIPHostnames map[string][]string
	}{
		{
			name:            "Simple valid hostfile",
			hostFileContent: []string{validHostFileContent},
			expectedHostnameIPs: map[string][]string{
				"localhost":     {"127.0.0.1"},
				"example.com":   {"192.168.1.10"},
				"test.local":    {"192.168.1.10"},
				"example2.com":  {"192.168.1.11"},
				"ip6-localhost": {"::1"},
				"ip6-loopback":  {"::1"},
			},
			expectedIPHostnames: map[string][]string{
				"127.0.0.1":    {"localhost"},
				"192.168.1.10": {"example.com", "test.local"},
				"192.168.1.11": {"example2.com"},
				"::1":          {"ip6-localhost", "ip6-loopback"},
			},
		},
		{
			name:            "Multiple hostfiles removing duplicates",
			hostFileContent: []string{validHostFileContent, validHostFileContent2},
			expectedHostnameIPs: map[string][]string{
				"localhost":           {"127.0.0.1"},
				"example.com":         {"192.168.1.10"},
				"test.local":          {"192.168.1.10"},
				"example2.com":        {"192.168.1.11", "192.168.1.10"},
				"ip6-localhost":       {"::1"},
				"ip6-loopback":        {"::1"},
				"another.example.com": {"192.168.1.20"},
				"test.example.com":    {"192.168.1.30"},
			},
			expectedIPHostnames: map[string][]string{
				"127.0.0.1":    {"localhost"},
				"192.168.1.10": {"example.com", "test.local", "example2.com"},
				"192.168.1.11": {"example2.com"},
				"192.168.1.20": {"another.example.com"},
				"192.168.1.30": {"test.example.com"},
				"::1":          {"ip6-localhost", "ip6-loopback"},
			},
		},
		{
			name:            "Hostfile with comments and empty lines",
			hostFileContent: []string{hostFileWithCommentsAndEmptyLines},
			expectedHostnameIPs: map[string][]string{
				"localhost":             {"127.0.0.1"},
				"commented.example.com": {"192.168.1.20"},
			},
			expectedIPHostnames: map[string][]string{
				"127.0.0.1":    {"localhost"},
				"192.168.1.20": {"commented.example.com"},
			},
		},
		{
			name:            "Hostfile with invalid entries",
			hostFileContent: []string{hostFileWithInvalidEntries},
			expectedHostnameIPs: map[string][]string{
				"valid.example.com": {"192.168.1.30"},
			},
			expectedIPHostnames: map[string][]string{
				"192.168.1.30": {"valid.example.com"},
			},
		},
		{
			name:                "Empty hostfile",
			hostFileContent:     []string{""},
			expectedHostnameIPs: map[string][]string{},
			expectedIPHostnames: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create host files
			filePaths := make([]string, len(tt.hostFileContent))
			for i, content := range tt.hostFileContent {
				filePaths[i] = testutil.CreateTempHostFile(t, content)
			}

			resolver, err := NewHostFileResolver(filePaths, logger)
			require.NoError(t, err)

			// Verify hostname to IP mappings
			for hostname, expectedIPs := range tt.expectedHostnameIPs {
				ips, found := resolver.hostnameToIP[hostname]
				assert.True(t, found, "Expected hostname %s not found", hostname)
				assert.ElementsMatch(t, expectedIPs, ips, "Mismatch for hostname: %s", hostname)
			}

			// Verify IP to hostname mappings
			for ip, expectedHostnames := range tt.expectedIPHostnames {
				hostnames, found := resolver.ipToHostname[ip]
				assert.True(t, found, "Expected IP %s not found", ip)
				assert.ElementsMatch(t, expectedHostnames, hostnames, "Mismatch for IP: %s", ip)
			}

			// Check that there are no extra entries
			assert.Len(t, resolver.hostnameToIP, len(tt.expectedHostnameIPs))
			assert.Len(t, resolver.ipToHostname, len(tt.expectedIPHostnames))
		})
	}
}

func TestHostFileResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	resolver := &HostFileResolver{
		hostnameToIP: map[string][]string{
			"localhost":    {"127.0.0.1"},
			"example.com":  {"192.168.1.10", "192.168.1.11"},
			"test.local":   {"192.168.1.20"},
			"ipv6.example": {"2001:db8::1"},
		},
		ipToHostname: map[string][]string{
			"127.0.0.1":    {"localhost"},
			"192.168.1.10": {"example.com"},
			"192.168.1.11": {"example.com"},
			"192.168.1.20": {"test.local"},
			"2001:db8::1":  {"ipv6.example"},
		},
		logger: logger,
	}

	tests := []struct {
		name        string
		hostname    string
		expectedIPs []string
		expectError bool
		expectedErr error
	}{
		{
			name:        "Valid hostname lookup",
			hostname:    "example.com",
			expectedIPs: []string{"192.168.1.10", "192.168.1.11"},
		},
		{
			name:        "Valid localhost lookup",
			hostname:    "localhost",
			expectedIPs: []string{"127.0.0.1"},
		},
		{
			name:        "Valid IPv6 hostname lookup",
			hostname:    "ipv6.example",
			expectedIPs: []string{"2001:db8::1"},
		},
		{
			name:        "Non-existent hostname",
			hostname:    "nonexistent.com",
			expectError: true,
			expectedErr: ErrNotInHostFiles,
		},
		{
			name:        "Empty hostname",
			hostname:    "",
			expectError: true,
			expectedErr: ErrNotInHostFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolver.Resolve(ctx, tt.hostname)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Empty(t, ips)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedIPs, ips)
			}
		})
	}
}

func TestHostFileResolver_Reverse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	resolver := &HostFileResolver{
		hostnameToIP: map[string][]string{
			"localhost":    {"127.0.0.1"},
			"example.com":  {"192.168.1.10"},
			"example2.com": {"192.168.1.10"},
			"test.local":   {"192.168.1.20"},
			"ipv6.example": {"2001:db8::1"},
		},
		ipToHostname: map[string][]string{
			"127.0.0.1":    {"localhost"},
			"192.168.1.10": {"example.com", "example2.com"},
			"192.168.1.20": {"test.local"},
			"2001:db8::1":  {"ipv6.example"},
		},
		logger: logger,
	}

	tests := []struct {
		name              string
		ip                string
		expectedHostnames []string
		expectError       bool
		expectedErr       error
	}{
		{
			name:              "Valid IPv4 lookup",
			ip:                "192.168.1.10",
			expectedHostnames: []string{"example.com", "example2.com"},
		},
		{
			name:              "Valid localhost lookup",
			ip:                "127.0.0.1",
			expectedHostnames: []string{"localhost"},
		},
		{
			name:              "Valid IPv6 lookup",
			ip:                "2001:db8::1",
			expectedHostnames: []string{"ipv6.example"},
		},
		{
			name:        "Non-existent IP",
			ip:          "192.168.1.100",
			expectError: true,
			expectedErr: ErrNotInHostFiles,
		},
		{
			name:        "Empty IP",
			ip:          "",
			expectError: true,
			expectedErr: ErrNotInHostFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostnames, err := resolver.Reverse(ctx, tt.ip)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Empty(t, hostnames)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedHostnames, hostnames)
			}
		})
	}
}
