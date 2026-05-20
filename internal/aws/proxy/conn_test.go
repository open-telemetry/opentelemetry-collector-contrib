// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func TestNewAWSConfigDelegatesToAwsutil(t *testing.T) {
	t.Run("without RoleARN", func(t *testing.T) {
		logger := zap.NewNop()
		t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

		cfg, err := newAWSConfig(t.Context(), "", "us-west-2", logger)
		assert.NoError(t, err)
		assert.Equal(t, "us-west-2", cfg.Region, "region should be set")
		assert.Equal(t, 2, cfg.RetryMaxAttempts, "max retries should be 2")
	})

	t.Run("with RoleARN", func(t *testing.T) {
		logger := zap.NewNop()
		t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

		cfg, err := newAWSConfig(t.Context(), "arn:aws:iam::123456789012:role/TestRole", "eu-west-1", logger)
		assert.NoError(t, err)
		assert.Equal(t, "eu-west-1", cfg.Region, "region should be set")
		assert.Equal(t, 2, cfg.RetryMaxAttempts, "max retries should be 2")
		assert.NotNil(t, cfg.Credentials, "credentials provider should be set for RoleARN")
	})

	// Verify SDK handles config creation for regions across all AWS partitions.
	// This ensures getServiceEndpoint (tested in server_test.go) and newAWSConfig
	// both work consistently for partition-specific regions.
	t.Run("all partitions", func(t *testing.T) {
		logger := zap.NewNop()
		t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

		partitionRegions := []struct {
			region    string
			partition string
		}{
			{"us-east-1", "aws"},
			{"eu-west-1", "aws"},
			{"cn-north-1", "aws-cn"},
			{"us-gov-west-1", "aws-us-gov"},
			{"us-iso-east-1", "aws-iso"},
			{"us-isob-east-1", "aws-iso-b"},
			{"eu-isoe-west-1", "aws-iso-e"},
			{"us-isof-south-1", "aws-iso-f"},
			{"eusc-de-east-1", "aws-eusc"},
		}

		for _, pr := range partitionRegions {
			t.Run(pr.partition+"/"+pr.region, func(t *testing.T) {
				cfg, err := newAWSConfig(t.Context(), "", pr.region, logger)
				assert.NoError(t, err, "should create config for %s", pr.region)
				assert.Equal(t, pr.region, cfg.Region)
			})
		}
	})
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
