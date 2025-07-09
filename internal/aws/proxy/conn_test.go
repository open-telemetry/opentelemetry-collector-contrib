// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var ec2Region = "us-west-2"

type mock struct {
	getEC2RegionErr error
	sn              *session.Session
}

func (m *mock) getEC2Region(_ *session.Session) (string, error) {
	if m.getEC2RegionErr != nil {
		return "", m.getEC2RegionErr
	}
	return ec2Region, nil
}

func (m *mock) newAWSSession(_ string, _ string, _ *zap.Logger) (*session.Session, error) {
	return m.sn, nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	return zap.New(core), recorded
}

func setupMock(sess *session.Session) (f1 func(s *session.Session) (string, error),
	f2 func(roleArn string, region string, logger *zap.Logger) (*session.Session, error),
) {
	f1 = getEC2Region
	f2 = newAWSSession
	m := mock{sn: sess}
	getEC2Region = m.getEC2Region
	newAWSSession = m.newAWSSession
	return
}

func tearDownMock(
	f1 func(s *session.Session) (string, error),
	f2 func(roleArn string, region string, logger *zap.Logger) (*session.Session, error),
) {
	getEC2Region = f1
	newAWSSession = f2
}

// fetch region value from environment variable
func TestRegionFromEnv(t *testing.T) {
	logger, recordedLogs := logSetup()
	region := "us-east-100"

	t.Setenv("AWS_REGION", region)

	expectedSession, err := session.NewSession()
	assert.NoError(t, err, "expectedSession should be created")
	f1, f2 := setupMock(expectedSession)
	defer tearDownMock(f1, f2)

	awsCfg, s, err := getAWSConfigSession(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, expectedSession, s, "mock session is not overridden")
	assert.Equal(t, region, *awsCfg.Region, "region value fetched from environment")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Fetched region from environment variables", "expected log message")
	assert.Equal(t, "region", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, region, lastEntry.Context[0].String)
}

// Get region from the config file
func TestRegionFromConfig(t *testing.T) {
	logger, recordedLogs := logSetup()

	expectedSession, err := session.NewSession()
	assert.NoError(t, err, "expectedSession should be created")
	f1, f2 := setupMock(expectedSession)
	defer tearDownMock(f1, f2)

	cfgWithRegion := DefaultConfig()
	cfgWithRegion.Region = "ap-northeast-1"

	awsCfg, s, err := getAWSConfigSession(cfgWithRegion, logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, expectedSession, s, "mock session is not overridden")
	assert.Equal(t, cfgWithRegion.Region, *awsCfg.Region, "region value fetched from the config file")

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

	expectedSession, err := session.NewSession()
	assert.NoError(t, err, "expectedSession should be created")
	f1, f2 := setupMock(expectedSession)
	defer tearDownMock(f1, f2)

	awsCfg, s, err := getAWSConfigSession(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, expectedSession, s, "mock session is not overridden")
	assert.Equal(t, "us-west-50", *awsCfg.Region, "region value fetched from ECS metadata")

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

	expectedSession, err := session.NewSession()
	assert.NoError(t, err, "expectedSession should be created")
	f1, f2 := setupMock(expectedSession)
	defer tearDownMock(f1, f2)

	_, s, err := getAWSConfigSession(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, expectedSession, s, "mock session is not overridden")

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

	expectedSession, err := session.NewSession()
	assert.NoError(t, err, "expectedSession should be created")
	f1, f2 := setupMock(expectedSession)
	defer tearDownMock(f1, f2)

	awsCfg, s, err := getAWSConfigSession(DefaultConfig(), logger)
	assert.NoError(t, err, "getAWSConfigSession should not error out")
	assert.Equal(t, expectedSession, s, "mock session is not overridden")
	assert.Equal(t, ec2Region, *awsCfg.Region, "region value fetched from ec2-metadata service")

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

	_, _, err := getAWSConfigSession(DefaultConfig(), logger)
	assert.Error(t, err, "getAWSConfigSession should fail")

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
		Val credentials.Value
	}{
		Env: map[string]string{
			"AWS_ACCESS_KEY":    "AKID",
			"AWS_SECRET_KEY":    "SECRET",
			"AWS_SESSION_TOKEN": "TOKEN",
		},
		Val: credentials.Value{
			AccessKeyID: "AKID", SecretAccessKey: "SECRET", SessionToken: "TOKEN",
			ProviderName: "EnvConfigCredentials",
		},
	}

	for k, v := range cases.Env {
		t.Setenv(k, v)
	}
	cfg, err := newAWSSession("", "", zap.NewNop())
	assert.NoError(t, err, "Expect no error")
	value, err := cfg.Config.Credentials.Get()
	assert.NoError(t, err, "Expect no error")
	assert.Equal(t, cases.Val, value, "Expect the credentials value to match")

	_, err = newAWSSession("ROLEARN", "TEST", zap.NewNop())
	assert.ErrorContains(t, err, "unable to handle AWS error", "expected error message")
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
	assert.Equal(t, endpoints.AwsPartitionID, p)

	p = getPartition("cn-north-1")
	assert.Equal(t, endpoints.AwsCnPartitionID, p)

	p = getPartition("us-gov-east-1")
	assert.Equal(t, endpoints.AwsUsGovPartitionID, p)

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

	p = getPartition("XYZ")
	assert.Empty(t, p)
}

func TestNewSessionCreationFailed(t *testing.T) {
	// manipulate env vars so that session.NewSession() fails
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "invalid")

	_, err := newAWSSession("", "dontCare", zap.NewNop())
	assert.Error(t, err, "expected failure")
}

func TestGetSTSCredsFailed(t *testing.T) {
	// manipulate env vars so that session.NewSession() fails
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "invalid")

	_, err := newAWSSession("ROLEARN", "us-west-2", zap.NewNop())
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
		getSTSCredsFromRegionEndpoint: func(_ *zap.Logger, _ *session.Session, region, roleArn string) *credentials.Credentials {
			assert.Equal(t, endpoints.UsEast1RegionID, region, "expected region differs")
			assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
			called = true
			return nil
		},
	}
	_, err := fake.getSTSCredsFromPrimaryRegionEndpoint(nil, expectedRoleARN, "us-west-2")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ *session.Session, region, roleArn string) *credentials.Credentials {
		assert.Equal(t, endpoints.CnNorth1RegionID, region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return nil
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(nil, expectedRoleARN, "cn-north-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ *session.Session, region, roleArn string) *credentials.Credentials {
		assert.Equal(t, endpoints.UsGovWest1RegionID, region, "expected region differs")
		assert.Equal(t, expectedRoleARN, roleArn, "expected role ARN differs")
		called = true
		return nil
	}
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(nil, expectedRoleARN, "us-gov-east-1")
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	called = false
	fake.getSTSCredsFromRegionEndpoint = func(_ *zap.Logger, _ *session.Session, _, _ string) *credentials.Credentials {
		called = true
		return nil
	}
	invalidRegion := "invalid region"
	_, err = fake.getSTSCredsFromPrimaryRegionEndpoint(nil, expectedRoleARN, invalidRegion)
	assert.False(t, called, "getSTSCredsFromRegionEndpoint should not be called")
	assert.EqualError(t, err,
		fmt.Sprintf("unrecognized AWS region: %s, or partition: ", invalidRegion),
		"expected error message")
}

type mockAWSErr struct{}

func (m *mockAWSErr) Error() string {
	return "mockAWSErr"
}

func (m *mockAWSErr) Code() string {
	return sts.ErrCodeRegionDisabledException
}

func (m *mockAWSErr) Message() string {
	return ""
}

func (m *mockAWSErr) OrigErr() error {
	return errors.New("mockAWSErr")
}

type mockProvider struct {
	retrieveErr error
}

func (m *mockProvider) Retrieve() (credentials.Value, error) {
	val, _ := credentials.AnonymousCredentials.Get()
	if m.retrieveErr != nil {
		return val, m.retrieveErr
	}
	return val, nil
}

func (m *mockProvider) IsExpired() bool {
	return true
}

func TestSTSRegionalEndpointDisabled(t *testing.T) {
	logger, recordedLogs := logSetup()

	const (
		expectedRoleARN = "a role ARN"
		expectedRegion  = "us-west-2000"
	)
	called := false
	expectedErr := &mockAWSErr{}
	fake := &stsCalls{
		log: logger,
		getSTSCredsFromRegionEndpoint: func(_ *zap.Logger, _ *session.Session, _, _ string) *credentials.Credentials {
			called = true
			return credentials.NewCredentials(&mockProvider{expectedErr})
		},
	}
	_, err := fake.getCreds(expectedRegion, expectedRoleARN)
	assert.True(t, called, "getSTSCredsFromRegionEndpoint should be called")
	assert.NoError(t, err, "no expected error")

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message,
		"STS regional endpoint disabled. Credentials for provided RoleARN will be fetched from STS primary region endpoint instead",
		"expected log message")
	assert.Equal(t,
		expectedRegion, lastEntry.Context[0].String, "expected error")
	assert.EqualError(t,
		lastEntry.Context[1].Interface.(error),
		expectedErr.Error(), "expected error")
}
