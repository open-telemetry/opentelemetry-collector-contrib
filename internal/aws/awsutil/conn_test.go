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

func (c *mockConn) getEC2Region(_ context.Context, _ aws.Config) (string, error) {
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
	assert.Empty(t, cfg.Region, "Region value should be empty when getEC2Region returns error")
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

// Test getPartition function with comprehensive test cases
func TestGetPartition(t *testing.T) {
	testCases := []struct {
		region   string
		expected string
		name     string
	}{
		// Standard AWS regions
		{region: "us-east-1", expected: "aws", name: "Standard AWS region (US East)"},
		{region: "eu-west-1", expected: "aws", name: "Standard AWS region (EU West)"},
		{region: "ap-northeast-1", expected: "aws", name: "Standard AWS region (AP Northeast)"},
		{region: "sa-east-1", expected: "aws", name: "Standard AWS region (SA East)"},
		{region: "ca-central-1", expected: "aws", name: "Standard AWS region (CA Central)"},
		{region: "me-south-1", expected: "aws", name: "Standard AWS region (ME South)"},
		{region: "af-south-1", expected: "aws", name: "Standard AWS region (AF South)"},
		{region: "il-central-1", expected: "aws", name: "Standard AWS region (IL Central)"},

		// China regions
		{region: "cn-north-1", expected: "aws-cn", name: "China region (North)"},
		{region: "cn-northwest-1", expected: "aws-cn", name: "China region (Northwest)"},

		// GovCloud regions
		{region: "us-gov-west-1", expected: "aws-us-gov", name: "GovCloud region (West)"},
		{region: "us-gov-east-1", expected: "aws-us-gov", name: "GovCloud region (East)"},

		// ISO regions
		{region: "us-iso-east-1", expected: "aws-iso", name: "ISO region (East)"},
		{region: "us-iso-west-1", expected: "aws-iso", name: "ISO region (West)"},

		// ISO-B regions
		{region: "us-isob-east-1", expected: "aws-iso-b", name: "ISO-B region (East)"},

		// ISO-E regions
		{region: "eu-isoe-west-1", expected: "aws-iso-e", name: "ISO-E region (West)"},

		// ISO-F regions
		{region: "us-isof-south-1", expected: "aws-iso-f", name: "ISO-F region (South)"},
		{region: "us-isof-east-1", expected: "aws-iso-f", name: "ISO-F region (East)"},

		// Edge cases
		{region: "", expected: "aws", name: "Empty region"},
		{region: "invalid-region", expected: "aws", name: "Invalid region"},
		{region: "us-iso", expected: "aws", name: "Partial ISO region name"},
		{region: "us-isof", expected: "aws", name: "Partial ISO-F region name"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getPartition(tc.region)
			assert.Equal(t, tc.expected, result, "Partition ID should match expected value")
		})
	}
}

// Test getSTSRegionalEndpoint function with comprehensive test cases
func TestGetSTSRegionalEndpoint(t *testing.T) {
	testCases := []struct {
		region   string
		expected string
		name     string
	}{
		// Standard AWS regions
		{region: "us-east-1", expected: STSEndpointPrefix + "us-east-1" + STSEndpointSuffix, name: "Standard AWS endpoint (US East)"},
		{region: "us-west-2", expected: STSEndpointPrefix + "us-west-2" + STSEndpointSuffix, name: "Standard AWS endpoint (US West)"},
		{region: "eu-west-1", expected: STSEndpointPrefix + "eu-west-1" + STSEndpointSuffix, name: "Standard AWS endpoint (EU West)"},

		// China regions
		{region: "cn-north-1", expected: STSEndpointPrefix + "cn-north-1" + STSAwsCnPartitionIDSuffix, name: "China endpoint (North)"},
		{region: "cn-northwest-1", expected: STSEndpointPrefix + "cn-northwest-1" + STSAwsCnPartitionIDSuffix, name: "China endpoint (Northwest)"},

		// GovCloud regions
		{region: "us-gov-west-1", expected: STSEndpointPrefix + "us-gov-west-1" + STSEndpointSuffix, name: "GovCloud endpoint (West)"},
		{region: "us-gov-east-1", expected: STSEndpointPrefix + "us-gov-east-1" + STSEndpointSuffix, name: "GovCloud endpoint (East)"},

		// ISO regions
		{region: "us-iso-east-1", expected: STSEndpointPrefix + "us-iso-east-1" + STSAwsIsoSuffix, name: "ISO endpoint (East)"},
		{region: "us-iso-west-1", expected: STSEndpointPrefix + "us-iso-west-1" + STSAwsIsoSuffix, name: "ISO endpoint (West)"},

		// ISO-B regions
		{region: "us-isob-east-1", expected: STSEndpointPrefix + "us-isob-east-1" + STSAwsIsoBSuffix, name: "ISO-B endpoint (East)"},

		// ISO-E regions
		{region: "eu-isoe-west-1", expected: STSEndpointPrefix + "eu-isoe-west-1" + STSAwsIsoESuffix, name: "ISO-E endpoint (West)"},

		// ISO-F regions
		{region: "us-isof-south-1", expected: STSEndpointPrefix + "us-isof-south-1" + STSAwsIsoFSuffix, name: "ISO-F endpoint (South)"},
		{region: "us-isof-east-1", expected: STSEndpointPrefix + "us-isof-east-1" + STSAwsIsoFSuffix, name: "ISO-F endpoint (East)"},

		// Edge cases
		{region: "invalid-region", expected: STSEndpointPrefix + "invalid-region" + STSEndpointSuffix, name: "Invalid region endpoint"},
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
	provider, err := CreateAssumeRoleCredentialProvider(cfg, "arn:aws:iam::123456789012:role/test-role", "")
	assert.NoError(t, err, "Should not return error")
	assert.NotNil(t, provider, "Credential provider should not be nil")
}

// Test with external ID
func TestCreateAssumeRoleCredentialProviderWithExternalID(t *testing.T) {
	cfg := aws.Config{}
	externalID := "external-id-123"
	provider, err := CreateAssumeRoleCredentialProvider(cfg, "arn:aws:iam::123456789012:role/test-role", externalID)
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
