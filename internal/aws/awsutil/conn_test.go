// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Test fetching region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	region := "us-east-1"
	t.Setenv("AWS_REGION", region)

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.NoError(t, err)
	assert.Equal(t, region, cfg.Region, "Region value should be fetched from environment")
}

// Test GetAWSConfig with explicit region setting
func TestGetAWSConfigWithExplicitRegion(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = "eu-west-1"

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.NoError(t, err)
	assert.Equal(t, "eu-west-1", cfg.Region, "Region value should match the explicitly set region")
}

// Test GetAWSConfig with custom endpoint
func TestGetAWSConfigWithCustomEndpoint(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = "us-west-2"
	sessionCfg.Endpoint = "https://custom.endpoint.com"

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", cfg.Region)
	assert.NotNil(t, cfg.BaseEndpoint)
	assert.Equal(t, "https://custom.endpoint.com", *cfg.BaseEndpoint)
}

// Test GetAWSConfig with retry settings
func TestGetAWSConfigWithRetries(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = "us-west-2"
	sessionCfg.MaxRetries = 5

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.NoError(t, err)
	assert.Equal(t, 5, cfg.RetryMaxAttempts)
}

// Test NewHTTPClient
func TestNewHTTPClient(t *testing.T) {
	logger := zap.NewNop()

	testCases := []struct {
		name         string
		proxyAddress string
		expectError  bool
	}{
		{
			name:         "Valid configuration without proxy",
			proxyAddress: "",
			expectError:  false,
		},
		{
			name:         "Valid configuration with proxy",
			proxyAddress: "http://example.com",
			expectError:  false,
		},
		{
			name:         "Invalid proxy URL",
			proxyAddress: "://invalid",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := newHTTPClient(logger, 8, 30, false, tc.proxyAddress)

			if tc.expectError {
				assert.Error(t, err, "Should return error with invalid proxy")
				assert.Nil(t, client, "Client should be nil on error")
			} else {
				assert.NoError(t, err, "Should not return error with valid configuration")
				assert.NotNil(t, client, "Client should not be nil")
			}
		})
	}
}

// Test HTTP client with no SSL verification
func TestNewHTTPClientNoVerifySSL(t *testing.T) {
	logger := zap.NewNop()
	client, err := newHTTPClient(logger, 8, 30, true, "")
	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, client, "Client should not be nil")

	// Check that the transport has InsecureSkipVerify set
	transport, ok := client.Transport.(*http.Transport)
	assert.True(t, ok, "Transport should be of type *http.Transport")
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "InsecureSkipVerify should be true")
}

// Test HTTP client with proxy from environment
func TestNewHTTPClientWithProxyFromEnv(t *testing.T) {
	logger := zap.NewNop()
	proxyURL := "http://env.proxy.com:8080"
	t.Setenv("HTTPS_PROXY", proxyURL)

	client, err := newHTTPClient(logger, 8, 30, false, "")
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// Test HTTP client with custom timeout
func TestNewHTTPClientWithCustomTimeout(t *testing.T) {
	logger := zap.NewNop()
	timeoutSeconds := 60

	client, err := newHTTPClient(logger, 8, timeoutSeconds, false, "")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 60*time.Second, client.Timeout)
}

// Test getProxyAddress function
func TestGetProxyAddress(t *testing.T) {
	testCases := []struct {
		name         string
		proxyAddress string
		envProxy     string
		expected     string
	}{
		{
			name:         "Explicit proxy address",
			proxyAddress: "http://explicit.proxy.com",
			envProxy:     "http://env.proxy.com",
			expected:     "http://explicit.proxy.com",
		},
		{
			name:         "Environment proxy address",
			proxyAddress: "",
			envProxy:     "http://env.proxy.com",
			expected:     "http://env.proxy.com",
		},
		{
			name:         "No proxy address",
			proxyAddress: "",
			envProxy:     "",
			expected:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envProxy != "" {
				t.Setenv("HTTPS_PROXY", tc.envProxy)
			} else {
				t.Setenv("HTTPS_PROXY", "")
			}

			result := getProxyAddress(tc.proxyAddress)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test getProxyURL function
func TestGetProxyURL(t *testing.T) {
	testCases := []struct {
		name        string
		proxyAddr   string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Valid proxy URL",
			proxyAddr:   "http://proxy.example.com:8080",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "Empty proxy address",
			proxyAddr:   "",
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "Invalid proxy URL",
			proxyAddr:   "://invalid",
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := getProxyURL(tc.proxyAddr)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectNil {
				assert.Nil(t, url)
			} else if !tc.expectError {
				assert.NotNil(t, url)
			}
		})
	}
}
