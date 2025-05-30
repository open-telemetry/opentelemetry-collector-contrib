// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Flip hostname to IP", LogKeyHostname, LogKeyIP},
		{"Flip IP to hostname", LogKeyIP, LogKeyHostname},
		{"Flip unknown key defaults to hostname", "unknown", LogKeyHostname},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Flip(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIP(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorType   error
	}{
		{
			name:        "Valid IPv4",
			input:       "192.168.1.1",
			expected:    "192.168.1.1",
			expectError: false,
		},
		{
			name:        "Valid IPv6",
			input:       "2001:db8::1",
			expected:    "2001:db8::1",
			expectError: false,
		},
		{
			name:        "Invalid IP",
			input:       "not-an-ip",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
		{
			name:        "Unspecified IPv4",
			input:       "0.0.0.0",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
		{
			name:        "Unspecified IPv6",
			input:       "::",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
		{
			name:        "IP with port (invalid)",
			input:       "192.168.1.1:80",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
		{
			name:        "Hostname",
			input:       "some-server.example.com",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseIP(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseHostname(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorType   error
	}{
		{
			name:        "Valid hostname",
			input:       "example.com",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "Valid hostname with subdomain",
			input:       "sub.example.com",
			expected:    "sub.example.com",
			expectError: false,
		},
		{
			name:        "Valid hostname with hyphen",
			input:       "some-server.example.com",
			expected:    "some-server.example.com",
			expectError: false,
		},
		{
			name:        "Valid hostname with numeric parts",
			input:       "server123.example.com",
			expected:    "server123.example.com",
			expectError: false,
		},
		{
			name:        "Valid localhost",
			input:       "localhost",
			expected:    "localhost",
			expectError: false,
		},
		{
			name:        "Invalid hostname with spaces",
			input:       "some server.example.com",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidHostname,
		},
		{
			name:        "Invalid hostname with special characters",
			input:       "server$.example.com",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidHostname,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidHostname,
		},
		{
			name:        "IP address",
			input:       "192.168.1.1",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidHostname,
		},
		{
			name:        "Hostname too long (over 255 characters)",
			input:       "a" + string(make([]byte, 255)) + ".com",
			expected:    "",
			expectError: true,
			errorType:   ErrInvalidHostname,
		},
		{
			name:        "Hostname with trailing dot",
			input:       "example.com.",
			expected:    "example.com.",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHostname(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRemoveTrailingDot(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Hostname with trailing dot",
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "Hostname without trailing dot",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single dot",
			input:    ".",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveTrailingDot(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Lowercase hostname",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "Uppercase hostname",
			input:    "EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "Mixed case hostname",
			input:    "ExAmPlE.CoM",
			expected: "example.com",
		},
		{
			name:     "Hostname with trailing dot",
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "Uppercase hostname with trailing dot",
			input:    "EXAMPLE.COM.",
			expected: "example.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Hostname with subdomain",
			input:    "Sub.Example.Com.",
			expected: "sub.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeHostname(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
