// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

var ec2Region = "us-west-2"

type mockConn struct {
	mock.Mock
}

func (c *mockConn) getEC2Region(ctx context.Context, cfg aws.Config) (string, error) {
	args := c.Called(nil)
	errorStr := args.String(0)
	var err error
	if errorStr != "" {
		err = errors.New(errorStr)
		return "", err
	}
	return ec2Region, nil
}

// fetch region value from ec2 meta data service
func TestEC2Session(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)

	// In SDK v2, we need to check Region field directly
	assert.Equal(t, "", cfg.Region, "Region value should be empty when getEC2Region returns error")
	assert.Error(t, err)
}

// fetch region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	region := "us-east-1"
	t.Setenv("AWS_REGION", region)

	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, logger, &sessionCfg)
	assert.Equal(t, region, cfg.Region, "Region value should be fetched from environment")
	assert.NoError(t, err)
}

// Test getPartition function
func TestGetPartition(t *testing.T) {
	testCases := []struct {
		region   string
		expected string
		name     string
	}{
		{region: "us-east-1", expected: "aws", name: "Standard AWS region"},
		{region: "cn-north-1", expected: "aws-cn", name: "China region"},
		{region: "us-gov-west-1", expected: "aws-us-gov", name: "GovCloud region"},
		{region: "us-iso-east-1", expected: "aws-iso", name: "ISO region"},
		{region: "us-isob-east-1", expected: "aws-iso-b", name: "ISO-B region"},
		{region: "eu-isoe-west-1", expected: "aws-iso-e", name: "ISO-E region"},
		{region: "us-isof-south-1", expected: "aws-iso-f", name: "ISO-F region"},
		{region: "", expected: "aws", name: "Empty region"},
		{region: "invalid-region", expected: "aws", name: "Invalid region"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getPartition(tc.region)
			assert.Equal(t, tc.expected, result, "Partition ID should match expected value")
		})
	}
}

// Test getSTSRegionalEndpoint function
func TestGetSTSRegionalEndpoint(t *testing.T) {
	testCases := []struct {
		region   string
		expected string
		name     string
	}{
		{region: "us-east-1", expected: STSEndpointPrefix + "us-east-1" + STSEndpointSuffix, name: "Standard AWS endpoint"},
		{region: "cn-north-1", expected: STSEndpointPrefix + "cn-north-1" + STSAwsCnPartitionIDSuffix, name: "China endpoint"},
		{region: "us-gov-west-1", expected: STSEndpointPrefix + "us-gov-west-1" + STSEndpointSuffix, name: "GovCloud endpoint"},
		{region: "us-iso-east-1", expected: STSEndpointPrefix + "us-iso-east-1" + STSAwsIsoSuffix, name: "ISO endpoint"},
		{region: "us-isob-east-1", expected: STSEndpointPrefix + "us-isob-east-1" + STSAwsIsoBSuffix, name: "ISO-B endpoint"},
		{region: "eu-isoe-west-1", expected: STSEndpointPrefix + "eu-isoe-west-1" + STSAwsIsoESuffix, name: "ISO-E endpoint"},
		{region: "us-isof-south-1", expected: STSEndpointPrefix + "us-isof-south-1" + STSAwsIsoFSuffix, name: "ISO-F endpoint"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getSTSRegionalEndpoint(tc.region)
			assert.Equal(t, tc.expected, result, "STS endpoint should match expected value")
		})
	}
}

// Test static credential provider
func TestCreateStaticCredentialProvider(t *testing.T) {
	provider := CreateStaticCredentialProvider("access", "secret", "token")
	assert.NotNil(t, provider, "Credential provider should not be nil")
}

// Test assume role credential provider
func TestCreateAssumeRoleCredentialProvider(t *testing.T) {
	cfg := aws.Config{}
	provider, err := CreateAssumeRoleCredentialProvider(context.Background(), cfg, "arn:aws:iam::123456789012:role/test-role", "")
	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, provider, "Credential provider should not be nil")
}

// Test with external ID
func TestCreateAssumeRoleCredentialProviderWithExternalID(t *testing.T) {
	cfg := aws.Config{}
	externalID := "external-id-123"
	provider, err := CreateAssumeRoleCredentialProvider(context.Background(), cfg, "arn:aws:iam::123456789012:role/test-role", externalID)
	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, provider, "Credential provider should not be nil")
}

// Test ProxyServerTransport
func TestProxyServerTransport(t *testing.T) {
	logger := zap.NewNop()
	config := &AWSSessionSettings{
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		ProxyAddress:          "http://example.com",
		NoVerifySSL:           false,
	}

	transport, err := ProxyServerTransport(logger, config)
	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, transport, "Transport should not be nil")
	assert.Equal(t, 8, transport.MaxIdleConns, "MaxIdleConns should match configuration")
	assert.Equal(t, 8, transport.MaxIdleConnsPerHost, "MaxIdleConnsPerHost should match configuration")
}

// Test NewHTTPClient
func TestNewHTTPClient(t *testing.T) {
	logger := zap.NewNop()

	// Test with valid configuration
	client, err := newHTTPClient(logger, 8, 30, false, "")
	assert.NoError(t, err, "Should not return error with valid configuration")
	assert.NotNil(t, client, "Client should not be nil")

	// Test with proxy
	client, err = newHTTPClient(logger, 8, 30, false, "http://example.com")
	assert.NoError(t, err, "Should not return error with proxy")
	assert.NotNil(t, client, "Client should not be nil")

	// Test with invalid proxy
	_, err = newHTTPClient(logger, 8, 30, false, "://invalid")
	assert.Error(t, err, "Should return error with invalid proxy")
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
