// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var ec2Region = "us-west-2"

type mock struct {
	getEC2RegionErr             error
	getRegionFromEC2MetadataErr error
	cfg                         aws.Config
}

func (m *mock) getEC2Region(_ context.Context, _ aws.Config) (string, error) {
	if m.getEC2RegionErr != nil {
		return "", m.getEC2RegionErr
	}
	return ec2Region, nil
}

func (m *mock) newAWSConfig(_ context.Context, _, region string, _ *zap.Logger) (aws.Config, error) {
	cfg := m.cfg
	// Mirror real awsutil.GetAWSConfig behavior: region is always set in the returned config
	if region != "" {
		cfg.Region = region
	}
	return cfg, nil
}

func (m *mock) getRegionFromEC2Metadata(_ context.Context, _ *zap.Logger) (string, error) {
	if m.getRegionFromEC2MetadataErr != nil {
		return "", m.getRegionFromEC2MetadataErr
	}
	return ec2Region, nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	return zap.New(core), recorded
}

func setupMock(t *testing.T, cfg aws.Config) {
	t.Helper()
	origGetEC2Region := getEC2Region
	origNewAWSConfig := newAWSConfig
	origGetRegionFromEC2Metadata := getRegionFromEC2Metadata

	m := mock{cfg: cfg}
	getEC2Region = m.getEC2Region
	newAWSConfig = m.newAWSConfig
	getRegionFromEC2Metadata = m.getRegionFromEC2Metadata

	t.Cleanup(func() {
		getEC2Region = origGetEC2Region
		newAWSConfig = origNewAWSConfig
		getRegionFromEC2Metadata = origGetRegionFromEC2Metadata
	})
}

// fetch region value from environment variable
func TestRegionFromEnv(t *testing.T) {
	logger, recordedLogs := logSetup()
	region := "us-east-100"

	t.Setenv("AWS_REGION", region)

	expectedCfg := aws.Config{Region: region}
	setupMock(t, expectedCfg)

	awsCfg, err := getAWSConfigSession(t.Context(), DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, region, awsCfg.Region, "region value fetched from environment")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from environment variables", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, region, lastEntry.Context[0].String)
}

// Get region from the config file
func TestRegionFromConfig(t *testing.T) {
	logger, recordedLogs := logSetup()

	expectedCfg := aws.Config{}
	setupMock(t, expectedCfg)

	cfgWithRegion := DefaultConfig()
	cfgWithRegion.Region = "ap-northeast-1"

	awsCfg, err := getAWSConfigSession(t.Context(), cfgWithRegion, logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, cfgWithRegion.Region, awsCfg.Region, "region value fetched from the config file")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from config file", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, cfgWithRegion.Region, lastEntry.Context[0].String)
}

func TestRegionFromECS(t *testing.T) {
	// Clear environment variables that might interfere
	t.Setenv(awsDefaultRegionEnvVar, "")
	t.Setenv(awsRegionEnvVar, "")
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/ecsmetadatafile.txt")

	logger, recordedLogs := logSetup()

	expectedCfg := aws.Config{}
	setupMock(t, expectedCfg)

	awsCfg, err := getAWSConfigSession(t.Context(), DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, "us-west-50", awsCfg.Region, "region value fetched from ECS metadata")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from ECS metadata file", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, "us-west-50", lastEntry.Context[0].String)
}

func TestRegionFromECSInvalidArn(t *testing.T) {
	// Clear environment variables that might interfere
	t.Setenv(awsDefaultRegionEnvVar, "")
	t.Setenv(awsRegionEnvVar, "")
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/ecsmetadatafileInvalidArn.txt")

	logger, recordedLogs := logSetup()

	expectedCfg := aws.Config{}
	setupMock(t, expectedCfg)

	_, err := getAWSConfigSession(t.Context(), DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")

	logs := recordedLogs.All()
	// Find the log entry about ECS metadata failure
	var foundECSError bool
	var foundEC2Fallback bool
	for _, entry := range logs {
		if strings.Contains(entry.Message, "Unable to fetch region from ECS metadata") {
			foundECSError = true
			assert.Error(t, entry.Context[0].Interface.(error), "expected error")
		}
		if strings.Contains(entry.Message, "Fetched region from EC2 metadata") {
			foundEC2Fallback = true
		}
	}
	assert.True(t, foundECSError, "expected ECS metadata error log")
	assert.True(t, foundEC2Fallback, "expected EC2 metadata fallback log")
}

// fetch region value from ec2 meta data service
func TestRegionFromEC2(t *testing.T) {
	// Clear environment variables that might interfere
	t.Setenv(awsDefaultRegionEnvVar, "")
	t.Setenv(awsRegionEnvVar, "")

	logger, recordedLogs := logSetup()

	expectedCfg := aws.Config{}
	setupMock(t, expectedCfg)

	awsCfg, err := getAWSConfigSession(t.Context(), DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, ec2Region, awsCfg.Region, "region value fetched from ec2-metadata service")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from EC2 metadata", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, lastEntry.Context[0].String, ec2Region)
}

func TestNoRegion(t *testing.T) {
	// Clear environment variables that might interfere
	t.Setenv(awsDefaultRegionEnvVar, "")
	t.Setenv(awsRegionEnvVar, "")

	logger, recordedLogs := logSetup()
	m := mock{
		getRegionFromEC2MetadataErr: errors.New("expected getEC2Region error"),
	}
	f1 := getRegionFromEC2Metadata
	getRegionFromEC2Metadata = m.getRegionFromEC2Metadata
	defer func() {
		getRegionFromEC2Metadata = f1
	}()

	_, err := getAWSConfigSession(t.Context(), DefaultConfig(), logger)
	assert.Error(t, err, "getAWSConfigSession should fail")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Unable to fetch region from EC2 metadata", "expected log message")
	assert.EqualError(t,
		lastEntry.Context[0].Interface.(error),
		m.getRegionFromEC2MetadataErr.Error(), "expected error")
}

// getRegionFromECSMetadata() returns an error if ECS metadata related env is not set
func TestNoECSMetadata(t *testing.T) {
	_, err := getRegionFromECSMetadata()
	assert.EqualError(t, err, "ECS metadata endpoint is inaccessible", "expected error")
}

// getRegionFromECSMetadata() throws an error when ECS metadata file cannot be parsed as valid JSON
func TestInvalidECSMetadata(t *testing.T) {
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/ecsmetadatafileinvalid.txt")

	_, err := getRegionFromECSMetadata()
	assert.EqualError(t, err,
		"invalid json in read ECS metadata file content, path: testdata/ecsmetadatafileinvalid.txt, error: invalid character 'i' looking for beginning of value",
		"expected error")
}

// getRegionFromECSMetadata() throws an error and returns an empty string when ECS metadata file cannot be opened
func TestMissingECSMetadataFile(t *testing.T) {
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/doesntExist.txt")

	_, err := getRegionFromECSMetadata()
	assert.Regexp(t,
		"^unable to open ECS metadata file, path: testdata/doesntExist.txt, error: open testdata/doesntExist.txt:",
		err,
		"expected error")
}

func TestGetPrimaryRegion(t *testing.T) {
	tests := []struct {
		region          string
		expectedPrimary string
	}{
		{"us-east-1", "us-east-1"},
		{"us-west-2", "us-east-1"},
		{"eu-west-1", "us-east-1"},
		{"ap-southeast-1", "us-east-1"},
		{"cn-north-1", "cn-north-1"},
		{"cn-northwest-1", "cn-north-1"},
		{"us-gov-west-1", "us-gov-west-1"},
		{"us-gov-east-1", "us-gov-west-1"},
		{"us-iso-east-1", "us-iso-east-1"},
		{"us-iso-west-1", "us-iso-east-1"},
		{"us-isob-east-1", "us-isob-east-1"},
		{"eu-isoe-west-1", "eu-isoe-west-1"},
		{"us-isof-south-1", "us-isof-south-1"},
		{"eusc-de-east-1", "eusc-de-east-1"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			primary, err := getPrimaryRegion(tt.region)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPrimary, primary)
		})
	}
}

func TestProxyServerTransportInvalidProxyAddr(t *testing.T) {
	_, err := proxyServerTransport(&Config{
		ProxyAddress: "invalid\n",
	})
	assert.ErrorContains(t, err, "invalid control character in URL")
}

func TestProxyServerTransportHappyCase(t *testing.T) {
	_, err := proxyServerTransport(&Config{
		ProxyAddress: "",
	})
	assert.NoError(t, err, "no expected error")
}

func TestGetSTSCredsFromPrimaryRegionEndpoint(t *testing.T) {
	const expectedRoleARN = "a role ARN"
	called := false

	cfg := aws.Config{}

	fake := &stsCallsV2{
		log: zap.NewNop(),
		getSTSCredsFromRegionEndpoint: func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
			assert.Equal(t, "us-east-1", region, "expected region differs")
			assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
			called = true
			return &mockCredentialsProvider{}
		},
	}
	_, err := fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "us-west-2")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "cn-north-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "cn-north-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "us-gov-west-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "us-gov-east-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	// Test iso partitions
	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "us-iso-east-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "us-iso-west-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "us-isob-east-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "us-isob-west-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "eu-isoe-west-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "eu-isoe-west-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "us-isof-south-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "us-isof-east-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "eusc-de-east-1", region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "eusc-de-east-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	// Test unknown region falls back to standard partition's primary region
	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ context.Context, _ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, "us-east-1", region, "unknown regions should fall back to us-east-1")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return &mockCredentialsProvider{}
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(t.Context(), cfg, expectedRoleARN, "ap-southeast-99")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")
}

// mockCredentialsProvider implements aws.CredentialsProvider for testing
type mockCredentialsProvider struct {
	retrieveErr error
}

func (m *mockCredentialsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	if m.retrieveErr != nil {
		return aws.Credentials{}, m.retrieveErr
	}
	return aws.Credentials{
		AccessKeyID:     "mockAccessKey",
		SecretAccessKey: "mockSecret",
		SessionToken:    "mockToken",
		Source:          "mockProvider",
		CanExpire:       true,
		Expires:         time.Now().Add(time.Hour),
	}, nil
}

func TestSTSRegionalEndpointDisabled(t *testing.T) {
	logger, recordedLogs := logSetup()

	const (
		expectedRoleARN = "a role ARN"
		expectedRegion  = "us-west-2000"
	)
	called := false
	expectedErr := &types.RegionDisabledException{Message: aws.String("Region is disabled")}

	cfg := aws.Config{}

	fake := &stsCallsV2{
		log: logger,
		getSTSCredsFromRegionEndpoint: func(_ context.Context, _ *zap.Logger, _ aws.Config, _, _ string) aws.CredentialsProvider {
			called = true
			return &mockCredentialsProvider{retrieveErr: expectedErr}
		},
	}
	_, err := fake.getCreds(t.Context(), cfg, expectedRegion, expectedRoleARN)
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message,
		"STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
		"expected log message")
	assert.Equal(t,
		expectedRegion, lastEntry.Context[0].String, "expected region")
}
