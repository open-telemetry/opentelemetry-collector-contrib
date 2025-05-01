// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// valid hosts file with uppercase and lowercase
const validHostFileContent = `
127.0.0.1 localhost
192.168.1.10 Example.com test.local
::1 ip6-localhost ip6-loopback
`

// valid hosts file
const validHostFileContent2 = `
192.168.1.20 another.example.com
192.168.1.30 test.example.com
`

// Hosts file with comments and empty lines
const hostFileWithCommentsAndEmptyLines = `
# This is a comment
127.0.0.1 localhost

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
			hostFilePaths: []string{createTempHostFile(t, validHostFileContent)},
			expectError:   false,
		},
		{
			name:          "Multiple valid hostfiles",
			hostFilePaths: []string{createTempHostFile(t, validHostFileContent), createTempHostFile(t, validHostFileContent2)},
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
			hostFilePaths: []string{createTempHostFile(t, validHostFileContent), "/non/existent/path"},
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

func TestHostFileResolver_ParseHostFile(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name                string
		hostFileContent     string
		expectedHostnameIPs map[string]string
		expectedIPHostnames map[string]string
	}{
		{
			name:            "Simple valid hostfile",
			hostFileContent: validHostFileContent,
			expectedHostnameIPs: map[string]string{
				"localhost":     "127.0.0.1",
				"example.com":   "192.168.1.10",
				"test.local":    "192.168.1.10",
				"ip6-localhost": "::1",
				"ip6-loopback":  "::1",
			},
			expectedIPHostnames: map[string]string{
				"127.0.0.1":    "localhost",
				"192.168.1.10": "test.local", // Last hostname of this IP
				"::1":          "ip6-loopback",
			},
		},
		{
			name:            "Hostfile with comments and empty lines",
			hostFileContent: hostFileWithCommentsAndEmptyLines,
			expectedHostnameIPs: map[string]string{
				"commented.example.com": "192.168.1.20",
				"localhost":             "127.0.0.1",
			},
			expectedIPHostnames: map[string]string{
				"127.0.0.1":    "localhost",
				"192.168.1.20": "commented.example.com",
			},
		},
		{
			name:            "Hostfile with invalid entries",
			hostFileContent: hostFileWithInvalidEntries,
			expectedHostnameIPs: map[string]string{
				"valid.example.com": "192.168.1.30",
			},
			expectedIPHostnames: map[string]string{
				"192.168.1.30": "valid.example.com",
			},
		},
		{
			name:                "Empty hostfile",
			hostFileContent:     "",
			expectedHostnameIPs: map[string]string{},
			expectedIPHostnames: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempHostFile := createTempHostFile(t, tt.hostFileContent)

			resolver := &HostFileResolver{
				name:         "hostfiles",
				hostnameToIP: make(map[string]string),
				ipToHostname: make(map[string]string),
				logger:       logger,
			}

			err := resolver.parseHostFile(tempHostFile)
			require.NoError(t, err)

			// Verify hostname to IP mappings
			for hostname, expectedIP := range tt.expectedHostnameIPs {
				ip, found := resolver.hostnameToIP[hostname]
				assert.True(t, found, "Expected hostname %s not found", hostname)
				assert.Equal(t, expectedIP, ip)
			}

			// Verify IP to hostname mappings
			for ip, expectedHostname := range tt.expectedIPHostnames {
				hostname, found := resolver.ipToHostname[ip]
				assert.True(t, found, "Expected IP %s not found", ip)
				assert.Equal(t, expectedHostname, hostname)
			}

			// Check that there are no extra entries
			assert.Len(t, tt.expectedHostnameIPs, len(resolver.hostnameToIP))
			assert.Len(t, tt.expectedIPHostnames, len(resolver.ipToHostname))
		})
	}
}

func TestHostFileResolver_Resolve(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	resolver := &HostFileResolver{
		name: "hostfiles",
		hostnameToIP: map[string]string{
			"localhost":    "127.0.0.1",
			"example.com":  "192.168.1.10",
			"test.local":   "192.168.1.20",
			"ipv6.example": "2001:db8::1",
		},
		ipToHostname: map[string]string{
			"127.0.0.1":    "localhost",
			"192.168.1.10": "example.com",
			"192.168.1.20": "test.local",
			"2001:db8::1":  "ipv6.example",
		},
		logger: logger,
	}

	tests := []struct {
		name          string
		hostname      string
		expectedIP    string
		expectError   bool
		expectedError error
	}{
		{
			name:        "Valid hostname lookup",
			hostname:    "example.com",
			expectedIP:  "192.168.1.10",
			expectError: false,
		},
		{
			name:        "Valid localhost lookup",
			hostname:    "localhost",
			expectedIP:  "127.0.0.1",
			expectError: false,
		},
		{
			name:        "Valid IPv6 hostname lookup",
			hostname:    "ipv6.example",
			expectedIP:  "2001:db8::1",
			expectError: false,
		},
		{
			name:          "Non-existent hostname",
			hostname:      "nonexistent.com",
			expectedIP:    "",
			expectError:   true,
			expectedError: ErrNotInHostFiles,
		},
		{
			name:          "Empty hostname",
			hostname:      "",
			expectedIP:    "",
			expectError:   true,
			expectedError: ErrNotInHostFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := resolver.Resolve(ctx, tt.hostname)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
				assert.Empty(t, ip)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}

func TestHostFileResolver_Reverse(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	// Create test resolver with predefined mappings
	resolver := &HostFileResolver{
		name: "hostfiles",
		hostnameToIP: map[string]string{
			"localhost":    "127.0.0.1",
			"example.com":  "192.168.1.10",
			"test.local":   "192.168.1.20",
			"ipv6.example": "2001:db8::1",
		},
		ipToHostname: map[string]string{
			"127.0.0.1":    "localhost",
			"192.168.1.10": "example.com",
			"192.168.1.20": "test.local",
			"2001:db8::1":  "ipv6.example",
		},
		logger: logger,
	}

	tests := []struct {
		name             string
		ip               string
		expectedHostname string
		expectError      bool
		expectedError    error
	}{
		{
			name:             "Valid IPv4 lookup",
			ip:               "192.168.1.10",
			expectedHostname: "example.com",
			expectError:      false,
		},
		{
			name:             "Valid localhost lookup",
			ip:               "127.0.0.1",
			expectedHostname: "localhost",
			expectError:      false,
		},
		{
			name:             "Valid IPv6 lookup",
			ip:               "2001:db8::1",
			expectedHostname: "ipv6.example",
			expectError:      false,
		},
		{
			name:             "Non-existent IP",
			ip:               "192.168.1.100",
			expectedHostname: "",
			expectError:      true,
			expectedError:    ErrNotInHostFiles,
		},
		{
			name:             "Empty IP",
			ip:               "",
			expectedHostname: "",
			expectError:      true,
			expectedError:    ErrNotInHostFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostname, err := resolver.Reverse(ctx, tt.ip)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
				assert.Empty(t, hostname)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHostname, hostname)
			}
		})
	}
}

// createTempHostFile create a temporary hostfile
func createTempHostFile(t *testing.T, content string) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "hosts")

	err := os.WriteFile(tempFile, []byte(content), 0o600)
	require.NoError(t, err)

	return tempFile
}
