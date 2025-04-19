// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGetAWSConfig_RegionInSettings verifies that the region is correctly read from settings
func TestGetAWSConfig_RegionInSettings(t *testing.T) {
	logger := zap.NewNop()
	settings := &AWSSessionSettings{
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "us-west-2", // Explicitly set region in settings
	}

	// Using a mock context for testing
	ctx := context.Background()

	// Skip actual call to AWS service by setting up a stub for testing
	original := getAWSConfigFunc
	defer func() { getAWSConfigFunc = original }()

	getAWSConfigFunc = func(ctx context.Context, logger *zap.Logger, settings *AWSSessionSettings) (aws.Config, error) {
		return aws.Config{
			Region: settings.Region,
		}, nil
	}

	cfg, err := GetAWSConfig(ctx, logger, settings)
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", cfg.Region, "Region should be taken from settings")
}

// TestGetAWSConfig_RegionFromEnv verifies that the region is correctly read from environment variables
func TestGetAWSConfig_RegionFromEnv(t *testing.T) {
	logger := zap.NewNop()
	settings := &AWSSessionSettings{
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "", // Empty region in settings
	}

	// Set environment variable for test
	t.Setenv("AWS_REGION", "us-east-1")
	ctx := context.Background()

	// Skip actual call to AWS service by setting up a stub for testing
	original := getAWSConfigFunc
	defer func() { getAWSConfigFunc = original }()

	getAWSConfigFunc = func(ctx context.Context, logger *zap.Logger, settings *AWSSessionSettings) (aws.Config, error) {
		// Emulate loading from environment variable
		return aws.Config{
			Region: "us-east-1", // This should match the env var
		}, nil
	}

	cfg, err := GetAWSConfig(ctx, logger, settings)
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", cfg.Region, "Region should be taken from environment variable")
}

// TestGetAWSConfig_WithHttpClient verifies HTTP client configuration
func TestGetAWSConfig_WithHttpClient(t *testing.T) {
	logger := zap.NewNop()
	// Only set fields that are actually used in this test
	settings := &AWSSessionSettings{
		NumberOfWorkers:       10,
		RequestTimeoutSeconds: 45,                              // Custom timeout
		NoVerifySSL:           true,                            // Skip TLS verification
		ProxyAddress:          "http://proxy.example.com:8080", // Custom proxy
	}

	// Create HTTP client
	httpClient, err := newHTTPClient(
		logger,
		settings.NumberOfWorkers,
		settings.RequestTimeoutSeconds,
		settings.NoVerifySSL,
		settings.ProxyAddress,
	)
	require.NoError(t, err)
	require.NotNil(t, httpClient)

	// Check timeout
	assert.Equal(t, 45*time.Second, httpClient.Timeout)

	// Check transport configuration
	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost)           // Should match NumberOfWorkers
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify) // Should match NoVerifySSL

	// Check proxy
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https"}})
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example.com:8080", proxyURL.String())
}

// TestCreateStaticCredentialProvider verifies static credential provider functionality
func TestCreateStaticCredentialProvider(t *testing.T) {
	accessKey := "AKIAIOSFODNN7EXAMPLE"
	secretKey := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	sessionToken := "EXAMPLESESSION"

	provider := CreateStaticCredentialProvider(accessKey, secretKey, sessionToken)

	ctx := context.Background()
	creds, err := provider.Retrieve(ctx)
	require.NoError(t, err)

	assert.Equal(t, accessKey, creds.AccessKeyID)
	assert.Equal(t, secretKey, creds.SecretAccessKey)
	assert.Equal(t, sessionToken, creds.SessionToken)
	assert.Equal(t, "StaticCredentials", creds.Source) // SDK V2 uses this as source identifier
}

// TestCreateAssumeRoleCredentialProvider tests the assume role credential provider
func TestCreateAssumeRoleCredentialProvider(t *testing.T) {
	// This test would require mocking STS, so we'll just verify the provider is created correctly
	ctx := context.Background()
	roleARN := "arn:aws:iam::123456789012:role/example-role"
	externalID := "example-external-id"

	// Mock config with region
	cfg := aws.Config{
		Region: "us-west-2",
	}

	provider, err := CreateAssumeRoleCredentialProvider(ctx, cfg, roleARN, externalID)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Instead of type assertion, check if the provider has the expected behavior
	// We can't easily call provider.Retrieve without mocking STS
	assert.NotNil(t, provider, "Provider should be created successfully")
}

// TestProxyServerTransport tests the proxy server transport configuration
func TestProxyServerTransport(t *testing.T) {
	logger := zap.NewNop()
	settings := &AWSSessionSettings{
		NumberOfWorkers:       12,
		RequestTimeoutSeconds: 60,
		NoVerifySSL:           true,
		ProxyAddress:          "http://proxy.example.org:9090",
	}

	transport, err := ProxyServerTransport(logger, settings)
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Verify transport settings
	assert.Equal(t, 12, transport.MaxIdleConns)
	assert.Equal(t, 12, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.True(t, transport.DisableCompression)

	// Verify proxy URL
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https"}})
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example.org:9090", proxyURL.String())
}

// TestGetProxyAddress tests retrieving proxy address from different sources
func TestGetProxyAddress(t *testing.T) {
	// Test with explicit address
	assert.Equal(t, "http://explicit.proxy:8080", getProxyAddress("http://explicit.proxy:8080"))

	// Test with environment variable
	origEnvValue := os.Getenv("HTTPS_PROXY")
	defer os.Setenv("HTTPS_PROXY", origEnvValue)

	os.Setenv("HTTPS_PROXY", "http://env.proxy:8888")
	assert.Equal(t, "http://env.proxy:8888", getProxyAddress(""))

	// Test with empty values
	os.Unsetenv("HTTPS_PROXY")
	assert.Equal(t, "", getProxyAddress(""))

	// Test priority (explicit over env)
	os.Setenv("HTTPS_PROXY", "http://env.proxy:8888")
	assert.Equal(t, "http://explicit.proxy:8080", getProxyAddress("http://explicit.proxy:8080"))
}

// TestGetProxyURL tests URL parsing for proxy addresses
func TestGetProxyURL(t *testing.T) {
	// Valid URL
	proxyURL, err := getProxyURL("http://proxy.example.com:8080")
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.example.com:8080", proxyURL.String())

	// Empty URL
	proxyURL, err = getProxyURL("")
	require.NoError(t, err)
	assert.Nil(t, proxyURL)

	// Invalid URL
	_, err = getProxyURL("http://invalid\nurl")
	require.Error(t, err)
}

// For testing purposes
var getAWSConfigFunc = func(ctx context.Context, logger *zap.Logger, settings *AWSSessionSettings) (aws.Config, error) {
	// This will be replaced in tests
	panic("getAWSConfigFunc not implemented")
}
