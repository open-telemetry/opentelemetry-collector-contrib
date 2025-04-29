// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var ec2Region = "us-west-2"

type mockConn struct {
	mock.Mock
	sn *session.Session
}

func (c *mockConn) getEC2Region(_ *session.Session, _ int) (string, error) {
	args := c.Called(nil)
	errorStr := args.String(0)
	var err error
	if errorStr != "" {
		err = errors.New(errorStr)
		return "", err
	}
	return ec2Region, nil
}

func (c *mockConn) newAWSSession(_ *zap.Logger, _ string, _ string, _ string) (*session.Session, error) {
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
	assert.Equal(t, expectedSession, s, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, ec2Region, "Region value fetched from ec2-metadata service")
	assert.NoError(t, err)
}

// fetch region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	sessionCfg := CreateDefaultSessionConfig()
	region := "us-east-1"
	t.Setenv("AWS_REGION", region)

	m := &mockConn{}
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, &sessionCfg)
	assert.Equal(t, expectedSession, s, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, region, "Region value fetched from environment")
	assert.NoError(t, err)
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
	assert.Error(t, err)
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
	assert.Error(t, err)
}

func TestNewAWSSessionWithErr(t *testing.T) {
	logger := zap.NewNop()
	roleArn := "fake_arn"
	region := "fake_region"
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	conn := &Conn{}
	aWSSessionSettings := &AWSSessionSettings{
		RoleARN: roleArn,
	}
	se, err := conn.newAWSSession(logger, aWSSessionSettings, region)
	assert.Error(t, err)
	assert.Nil(t, se)
	aWSSessionSettings = &AWSSessionSettings{
		RoleARN: "",
	}
	se, err = conn.newAWSSession(logger, aWSSessionSettings, region)
	assert.Error(t, err)
	assert.Nil(t, se)
	t.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "regional")
	se, _ = session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	assert.NotNil(t, se)
	_, err = conn.getEC2Region(se, aWSSessionSettings.IMDSRetries)
	assert.Error(t, err)
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
	aWSSessionSettings := &AWSSessionSettings{}
	_, err := GetDefaultSession(logger, aWSSessionSettings)
	assert.Error(t, err)
}

func TestGetSTSCreds(t *testing.T) {
	logger := zap.NewNop()
	region := "fake_region"
	roleArn := ""
	aWSSessionSettings := &AWSSessionSettings{
		RoleARN: roleArn,
	}
	creds, err := getSTSCreds(logger, region, aWSSessionSettings)
	assert.NotNil(t, creds)
	assert.NoError(t, err)
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	_, err = getSTSCreds(logger, region, aWSSessionSettings)
	assert.Error(t, err)
}

func TestLoadAmazonCertificateFromFile(t *testing.T) {
	certFromFile, err := loadCertPool("testdata/public_amazon_cert.pem")
	assert.NoError(t, err)
	assert.NotNil(t, certFromFile)
}

func TestLoadEmptyFile(t *testing.T) {
	certFromFile, err := loadCertPool("")
	assert.Error(t, err)
	assert.Nil(t, certFromFile)
}

func TestConfusedDeputyHeaders(t *testing.T) {
	tests := []struct {
		name                  string
		envSourceArn          string
		envSourceAccount      string
		expectedHeaderArn     string
		expectedHeaderAccount string
	}{
		{
			name:                  "unpopulated",
			envSourceArn:          "",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		{
			name:                  "both populated",
			envSourceArn:          "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			envSourceAccount:      "539247453986",
			expectedHeaderArn:     "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			expectedHeaderAccount: "539247453986",
		},
		{
			name:                  "only source arn populated",
			envSourceArn:          "arn:aws:ec2:us-east-1:474668408639:instance/i-08293cd9825754f7c",
			envSourceAccount:      "",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
		{
			name:                  "only source account populated",
			envSourceArn:          "",
			envSourceAccount:      "539247453986",
			expectedHeaderArn:     "",
			expectedHeaderAccount: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(AmzSourceAccount, tt.envSourceAccount)
			t.Setenv(AmzSourceArn, tt.envSourceArn)

			client := newStsClient(awsmock.Session, &aws.Config{
				// These are examples credentials pulled from:
				// https://docs.aws.amazon.com/STS/latest/APIReference/API_GetAccessKeyInfo.html
				Credentials: credentials.NewStaticCredentials("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
				Region:      aws.String("us-east-1"),
			})

			request, _ := client.AssumeRoleRequest(&sts.AssumeRoleInput{
				// We aren't going to actually make the assume role call, we are just going
				// to verify the headers are present once signed so the RoleArn and RoleSessionName
				// arguments are irrelevant. Fill them out with something so the request is valid.
				RoleArn:         aws.String("arn:aws:iam::012345678912:role/XXXXXXXX"),
				RoleSessionName: aws.String("MockSession"),
			})

			// Headers are generated after the request is signed (but before it's sent)
			err := request.Sign()
			require.NoError(t, err)

			headerSourceArn := request.HTTPRequest.Header.Get(SourceArnHeaderKey)
			assert.Equal(t, tt.expectedHeaderArn, headerSourceArn)

			headerSourceAccount := request.HTTPRequest.Header.Get(SourceAccountHeaderKey)
			assert.Equal(t, tt.expectedHeaderAccount, headerSourceAccount)
		})
	}
}
