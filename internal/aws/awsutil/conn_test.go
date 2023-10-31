// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

var ec2Region = "us-west-2"

type mockConn struct {
	mock.Mock
	sn *session.Session
}

func (c *mockConn) getEC2Region(_ *session.Session) (string, error) {
	args := c.Called(nil)
	errorStr := args.String(0)
	var err error
	if errorStr != "" {
		err = errors.New(errorStr)
		return "", err
	}
	return ec2Region, nil
}

func (c *mockConn) newAWSSession(_ *zap.Logger, _ string, _ string) (*session.Session, error) {
	return c.sn, nil
}

// fetch region value from ec2 meta data service
func TestEC2Session(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, &sessionCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, ec2Region, "Region value fetched from ec2-metadata service")
	assert.Nil(t, err)
}

// fetch region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	region := "us-east-1"
	t.Setenv("AWS_REGION", region)

	var m = &mockConn{}
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, &sessionCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, region, "Region value fetched from environment")
	assert.Nil(t, err)
}

func TestGetAWSConfigSessionWithSessionErr(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = ""
	sessionCfg.NoVerifySSL = false
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, &sessionCfg)
	assert.Nil(t, cfg)
	assert.Nil(t, s)
	assert.NotNil(t, err)
}

func TestGetAWSConfigSessionWithEC2RegionErr(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	sessionCfg.Region = ""
	sessionCfg.NoVerifySSL = false
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("some error").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, &sessionCfg)
	assert.Nil(t, cfg)
	assert.Nil(t, s)
	assert.NotNil(t, err)
}

func TestNewAWSSessionWithErr(t *testing.T) {
	logger := zap.NewNop()
	roleArn := "fake_arn"
	region := "fake_region"
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	conn := &Conn{}
	se, err := conn.newAWSSession(logger, roleArn, region)
	assert.NotNil(t, err)
	assert.Nil(t, se)
	roleArn = ""
	se, err = conn.newAWSSession(logger, roleArn, region)
	assert.NotNil(t, err)
	assert.Nil(t, se)
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "regional")
	se, _ = session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	assert.NotNil(t, se)
	_, err = conn.getEC2Region(se)
	assert.NotNil(t, err)
}

func TestGetSTSCredsFromPrimaryRegionEndpoint(t *testing.T) {
	logger := zap.NewNop()
	session, _ := session.NewSession()

	regions := []string{"us-east-1", "us-gov-west-1", "cn-north-1"}

	for _, region := range regions {
		creds := getSTSCredsFromPrimaryRegionEndpoint(logger, session, "", region)
		assert.NotNil(t, creds)
	}
	creds := getSTSCredsFromPrimaryRegionEndpoint(logger, session, "", "fake_region")
	assert.Nil(t, creds)
}

func TestGetDefaultSession(t *testing.T) {
	logger := zap.NewNop()
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	_, err := GetDefaultSession(logger)
	assert.NotNil(t, err)
}

func TestGetSTSCreds(t *testing.T) {
	logger := zap.NewNop()
	region := "fake_region"
	roleArn := ""
	_, err := getSTSCreds(logger, region, roleArn)
	assert.Nil(t, err)
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	_, err = getSTSCreds(logger, region, roleArn)
	assert.NotNil(t, err)
}
