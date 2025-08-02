// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var ec2Region = "us-west-2"

type mock struct {
	getEC2RegionErr error
	cfg             aws.Config
}

func (m *mock) getEC2Region(_ aws.Config) (string, error) {
	if m.getEC2RegionErr != nil {
		return "", m.getEC2RegionErr
	}
	return ec2Region, nil
}

func (m *mock) newAWSConfig(string, string, *zap.Logger) (aws.Config, error) {
	return m.cfg, nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	return zap.New(core), recorded
}

func setupMock(cfg aws.Config) (f1 func(cfg aws.Config) (string, error),
	f2 func(roleArn, region string, logger *zap.Logger) (aws.Config, error),
) {
	f1 = getEC2Region
	f2 = newAWSConfig
	m := mock{cfg: cfg}
	getEC2Region = m.getEC2Region
	newAWSConfig = m.newAWSConfig
	return
}

func tearDownMock(
	f1 func(cfg aws.Config) (string, error),
	f2 func(roleArn, region string, logger *zap.Logger) (aws.Config, error),
) {
	getEC2Region = f1
	newAWSConfig = f2
}

// fetch region value from environment variable
func TestRegionFromEnv(t *testing.T) {
	logger, recordedLogs := logSetup()
	region := "us-east-100"

	t.Setenv("AWS_REGION", region)

	expectedConfig := aws.Config{Region: region}
	f1, f2 := setupMock(expectedConfig)
	defer tearDownMock(f1, f2)

	awsCfg, err := getAWSConfig(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfig should not error out")
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

	cfgWithRegion := DefaultConfig()
	cfgWithRegion.Region = "ap-northeast-1"

	expectedConfig := aws.Config{Region: cfgWithRegion.Region}
	f1, f2 := setupMock(expectedConfig)
	defer tearDownMock(f1, f2)

	awsCfg, err := getAWSConfig(cfgWithRegion, logger)
	assert.NoError(t, err, "getAWSConfig should not error out")
	assert.Equal(t, cfgWithRegion.Region, awsCfg.Region, "region value fetched from the config file")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from config file", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, cfgWithRegion.Region, lastEntry.Context[0].String)
}

func TestRegionFromECS(t *testing.T) {
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/ecsmetadatafile.txt")

	logger, recordedLogs := logSetup()

	expectedConfig := aws.Config{Region: "us-west-50"}
	f1, f2 := setupMock(expectedConfig)
	defer tearDownMock(f1, f2)

	awsCfg, err := getAWSConfig(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfig should not error out")
	assert.Equal(t, "us-west-50", awsCfg.Region, "region value fetched from ECS metadata")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from ECS metadata file", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, "us-west-50", lastEntry.Context[0].String)
}

func TestRegionFromECSInvalidArn(t *testing.T) {
	t.Setenv(ecsContainerMetadataEnabledEnvVar, "true")
	t.Setenv(ecsMetadataFileEnvVar, "testdata/ecsmetadatafileInvalidArn.txt")

	logger, recordedLogs := logSetup()

	expectedConfig := aws.Config{Region: ec2Region}
	f1, f2 := setupMock(expectedConfig)
	defer tearDownMock(f1, f2)

	_, err := getAWSConfig(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfig should not error out")

	logs := recordedLogs.All()
	// assert fetching from ECS metadata failed
	sndToLastEntry := logs[len(logs)-2]
	assert.Contains(t, sndToLastEntry.Message, "Unable to fetch region from ECS metadata", "expected log message")
	assert.Error(t, sndToLastEntry.Context[0].Interface.(error), "expected error")

	// fall back to use EC2 meta data service
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from EC2 metadata", "expected log message")
}

// fetch region value from ec2 meta data service
func TestRegionFromEC2(t *testing.T) {
	logger, recordedLogs := logSetup()

	expectedConfig := aws.Config{Region: ec2Region}
	f1, f2 := setupMock(expectedConfig)
	defer tearDownMock(f1, f2)

	awsCfg, err := getAWSConfig(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfig should not error out")
	assert.Equal(t, ec2Region, awsCfg.Region, "region value fetched from ec2-metadata service")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from EC2 metadata", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, lastEntry.Context[0].String, ec2Region)
}

func TestNoRegion(t *testing.T) {
	logger, recordedLogs := logSetup()
	m := mock{
		getEC2RegionErr: errors.New("expected getEC2Region error"),
	}
	f1 := getEC2Region
	getEC2Region = m.getEC2Region
	defer func() {
		getEC2Region = f1
	}()

	_, err := getAWSConfig(DefaultConfig(), logger)
	assert.Error(t, err, "getAWSConfig should fail")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Unable to fetch region from EC2 metadata", "expected log message")
	assert.EqualError(t,
		lastEntry.Context[0].Interface.(error),
		m.getEC2RegionErr.Error(), "expected error")
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

func TestLoadEnvConfigCreds(t *testing.T) {
	cases := struct {
		Env map[string]string
		Val aws.Credentials
	}{
		Env: map[string]string{
			"AWS_ACCESS_KEY":    "AKID",
			"AWS_SECRET_KEY":    "SECRET",
			"AWS_SESSION_TOKEN": "TOKEN",
		},
		Val: aws.Credentials{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "TOKEN",
			Source: "EnvConfigCredentials",
		},
	}

	for k, v := range cases.Env {
		t.Setenv(k, v)
	}
	cfg, err := newAWSConfig("", "", zap.NewNop())
	assert.NoError(t, err, "Expect no error")
	value, err := cfg.Credentials.Retrieve(context.Background())
	assert.NoError(t, err, "Expect no error")
	assert.Equal(t, cases.Val, value, "Expect the credentials value to match")

	_, err = newAWSConfig("ROLEARN", "TEST", zap.NewNop())
	assert.Error(t, err, "unable to retrieve STS credentials", "expected error")
}

func TestGetProxyUrlProxyAddressNotValid(t *testing.T) {
	errorAddress := [3]string{"http://[%10::1]", "http://%41:8080/", "http://a b.com/"}
	for _, address := range errorAddress {
		_, err := getProxyURL(address)
		assert.Error(t, err, "expected error")
	}
}

func TestGetProxyAddressFromEnvVariable(t *testing.T) {
	t.Setenv(httpsProxyEnvVar, "https://127.0.0.1:8888")

	assert.Equal(t, os.Getenv(httpsProxyEnvVar), getProxyAddress(""), "Expect function return value should be same with Environment value")
}

func TestGetProxyAddressFromConfigFile(t *testing.T) {
	const expectedAddr = "https://127.0.0.1:8888"

	assert.Equal(t, expectedAddr, getProxyAddress("https://127.0.0.1:8888"), "Expect function return value should be same with input value")
}

func TestGetProxyAddressWhenNotExist(t *testing.T) {
	assert.Empty(t, getProxyAddress(""), "Expect function return value to be empty")
}

func TestGetProxyAddressPriority(t *testing.T) {
	t.Setenv(httpsProxyEnvVar, "https://127.0.0.1:8888")

	assert.Equal(t, "https://127.0.0.1:9999", getProxyAddress("https://127.0.0.1:9999"), "Expect function return value to be same with input")
}

func TestGetPartition(t *testing.T) {
	p := getPartition("us-east-1")
	assert.Equal(t, awsPartition, p)

	p = getPartition("cn-north-1")
	assert.Equal(t, awsChinaPartition, p)

	p = getPartition("us-gov-east-1")
	assert.Equal(t, awsGovCloudPartition, p)

	p = getPartition("XYZ")
	assert.Empty(t, p)
}

func TestGetSTSRegionalEndpoint(t *testing.T) {
	p := getSTSRegionalEndpoint("us-east-1")
	assert.Equal(t, "https://sts.us-east-1.amazonaws.com", p)

	p = getSTSRegionalEndpoint("cn-north-1")
	assert.Equal(t, "https://sts.cn-north-1.amazonaws.com.cn", p)

	p = getSTSRegionalEndpoint("us-gov-east-1")
	assert.Equal(t, "https://sts.us-gov-east-1.amazonaws.com", p)

	p = getSTSRegionalEndpoint("invalid-region")
	assert.Equal(t, "https://sts.invalid-region.amazonaws.com", p)
}

func TestNewSessionCreationFailed(t *testing.T) {
	// manipulate env vars so that session.NewSession() fails
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "invalid")

	_, err := newAWSConfig("", "dontCare", zap.NewNop())
	assert.NoError(t, err, "SDK v2 accepts this configuration")
}

func TestGetSTSCredsFailed(t *testing.T) {
	// manipulate env vars so that session.NewSession() fails
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "invalid")

	_, err := newAWSConfig("ROLEARN", "us-west-2", zap.NewNop())
	assert.Error(t, err, "expected failure")
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
	fake := &stsCalls{
		log: zap.NewNop(),
		getSTSCredsFromRegionEndpoint: func(_ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
			assert.Equal(t, awsPrimaryRegion, region, "expected region differs")
			assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
			called = true
			return nil
		},
	}
	_, err := fake.getSTSCredsFromPrimaryRegionEndpoint(aws.Config{}, expectedRoleARN, "us-west-2")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, awsChinaPrimaryRegion, region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return nil
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(aws.Config{}, expectedRoleARN, "cn-north-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ aws.Config, region, roleArn string) aws.CredentialsProvider {
		assert.Equal(t, awsGovCloudPrimaryRegion, region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return nil
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(aws.Config{}, expectedRoleARN, "us-gov-east-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ aws.Config, _, _ string) aws.CredentialsProvider {
		called = true
		return nil
	}
	invalidRegion := "invalid region"
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(aws.Config{}, expectedRoleARN, invalidRegion)
	assert.False(t, called, "getSTSCredsFromRegionEndpoint should not be called")
	assert.EqualError(t, err,
		fmt.Sprintf("unrecognized AWS region: %s, or partition: ", invalidRegion),
		"expected error message")
}

type mockProvider struct {
	retrieveErr error
}

func (m *mockProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	val := aws.Credentials{AccessKeyID: "ANON", SecretAccessKey: "ANON"}
	if m.retrieveErr != nil {
		return val, m.retrieveErr
	}
	return val, nil
}

func TestSTSRegionalEndpointDisabled(t *testing.T) {
	logger, recordedLogs := logSetup()

	const (
		expectedRoleARN = "a role ARN"
		expectedRegion  = "us-west-2000"
	)
	called := false
	expectedErr := &types.RegionDisabledException{
		Message: aws.String("mockAWSErr"),
	}
	fake := &stsCalls{
		log: logger,
		getSTSCredsFromRegionEndpoint: func(_ *zap.Logger, _ aws.Config, _, _ string) aws.CredentialsProvider {
			called = true
			return aws.NewCredentialsCache(&mockProvider{expectedErr})
		},
	}
	_, err := fake.getCreds(expectedRegion, expectedRoleARN, aws.Config{})
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "should succeed after fallback to primary region")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message,
		"STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
		"expected log message")
	assert.Equal(t,
		expectedRegion, lastEntry.Context[0].String, "expected region in log")
	assert.Contains(t,
		lastEntry.Context[1].Interface.(error).Error(),
		expectedErr.Error(), "expected original error in wrapped error")
}
