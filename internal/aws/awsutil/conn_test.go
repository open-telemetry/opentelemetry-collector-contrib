// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"errors"
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
	cfg, err := GetAWSConfigSession(ctx, logger, m, &sessionCfg)

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

	m := &mockConn{}
	ctx := context.Background()
	cfg, err := GetAWSConfigSession(ctx, logger, m, &sessionCfg)
	assert.Equal(t, region, cfg.Region, "Region value should be fetched from environment")
	assert.NoError(t, err)
}

// Test getPartition function
func TestGetPartition(t *testing.T) {
	// Test standard AWS region
	assert.Equal(t, "aws", getPartition("us-east-1"))

	// Test China region
	assert.Equal(t, "aws-cn", getPartition("cn-north-1"))

	// Test GovCloud region
	assert.Equal(t, "aws-us-gov", getPartition("us-gov-west-1"))

	// Test empty region
	assert.Equal(t, "aws", getPartition(""))
}

// Test getSTSRegionalEndpoint function
func TestGetSTSRegionalEndpoint(t *testing.T) {
	// Test standard AWS region endpoint
	assert.Equal(t, "https://sts.us-east-1.amazonaws.com", getSTSRegionalEndpoint("us-east-1"))

	// Test China region endpoint
	assert.Equal(t, "https://sts.cn-north-1.amazonaws.com.cn", getSTSRegionalEndpoint("cn-north-1"))

	// Test GovCloud region endpoint
	assert.Equal(t, "https://sts.us-gov-west-1.amazonaws.com", getSTSRegionalEndpoint("us-gov-west-1"))
}

// Test static credential provider
func TestCreateStaticCredentialProvider(t *testing.T) {
	provider := CreateStaticCredentialProvider("access", "secret", "token")
	assert.NotNil(t, provider)
}

// Test assume role credential provider
func TestCreateAssumeRoleCredentialProvider(t *testing.T) {
	cfg := aws.Config{}
	provider, err := CreateAssumeRoleCredentialProvider(context.Background(), cfg, "arn:aws:iam::123456789012:role/test-role", "")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
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
	assert.NoError(t, err)
	assert.NotNil(t, transport)
	assert.Equal(t, 8, transport.MaxIdleConns)
	assert.Equal(t, 8, transport.MaxIdleConnsPerHost)
}

// Test NewHTTPClient
func TestNewHTTPClient(t *testing.T) {
	logger := zap.NewNop()

	// Test with valid configuration
	client, err := newHTTPClient(logger, 8, 30, false, "")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test with proxy
	client, err = newHTTPClient(logger, 8, 30, false, "http://example.com")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test with invalid proxy
	_, err = newHTTPClient(logger, 8, 30, false, "://invalid")
	assert.Error(t, err)
}
