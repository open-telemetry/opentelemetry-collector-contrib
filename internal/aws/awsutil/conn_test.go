// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockSTS struct{}

func TestGetAWSConfig(t *testing.T) {
	logger := zap.NewNop()
	settings := &AWSSessionSettings{
		Region:   "us-mock-1",
		RoleARN:  "arn:aws:iam::123456789012:role/TestRole",
		Endpoint: "http://localhost:4566",
	}

	mockSTS := &mockSTS{}

	cfg, err := getAWSConfig(t.Context(), logger, settings, func(_ aws.Config) stscreds.AssumeRoleAPIClient {
		return mockSTS
	})

	assert.NoError(t, err)
	assert.Equal(t, "us-mock-1", cfg.Region)
	assert.Equal(t, "http://localhost:4566", *cfg.BaseEndpoint)
	assert.NotNil(t, cfg.Credentials)
}

func (*mockSTS) AssumeRole(_ context.Context, _ *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return &sts.AssumeRoleOutput{
		Credentials: &types.Credentials{
			AccessKeyId:     aws.String("mockAccessKey"),
			SecretAccessKey: aws.String("mockSecret"),
			SessionToken:    aws.String("mockToken"),
		},
	}, nil
}

// Test fetching region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	region := "us-east-1"
	t.Setenv("AWS_REGION", region)

	ctx := t.Context()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.NoError(t, err)
	assert.Equal(t, region, cfg.Region, "Region value should be fetched from environment")
}

// Test GetAWSConfig with explicit region setting
func TestGetAWSConfigWithExplicitRegion(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = "eu-west-1"

	ctx := t.Context()
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

	ctx := t.Context()
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

	ctx := t.Context()
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

func TestNoProxyIsRespectedWhenUsingEnvProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://mitmproxy:8080")
	t.Setenv("NO_PROXY", ".amazonaws.com,localhost,127.0.0.1")

	logger := zap.NewNop()

	client, err := newHTTPClient(logger, 8, 30, false, "")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "Transport should be *http.Transport")
	require.NotNil(t, transport.Proxy, "Proxy function should not be nil")

	req, err := http.NewRequest(http.MethodGet, "https://logs.eu-west-1.amazonaws.com/", http.NoBody)
	require.NoError(t, err)

	proxyURL, err := transport.Proxy(req)
	assert.NoError(t, err)
	assert.Nil(t, proxyURL,
		"Proxy should be nil for hosts matching NO_PROXY, but got: %v — "+
			"this proves NO_PROXY is not respected", proxyURL)
}

func TestProxyIsUsedForNonExcludedHosts(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://mitmproxy:8080")
	t.Setenv("NO_PROXY", ".amazonaws.com,localhost,127.0.0.1")

	logger := zap.NewNop()
	client, err := newHTTPClient(logger, 8, 30, false, "")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	req, err := http.NewRequest(http.MethodGet, "https://example.com/something", http.NoBody)
	require.NoError(t, err)

	proxyURL, err := transport.Proxy(req)
	assert.NoError(t, err)
	assert.NotNil(t, proxyURL)
	assert.Equal(t, "http://mitmproxy:8080", proxyURL.String())
}

func TestExplicitProxyAddressStillWorks(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://env.proxy:8080")
	logger := zap.NewNop()
	client, err := newHTTPClient(logger, 8, 30, false, "http://explicit.proxy:9090")
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	req, err := http.NewRequest(http.MethodGet, "https://example.com/", http.NoBody)
	require.NoError(t, err)

	proxyURL, err := transport.Proxy(req)
	assert.NoError(t, err)
	assert.NotNil(t, proxyURL)
	assert.Equal(t, "http://explicit.proxy:9090", proxyURL.String())
}
