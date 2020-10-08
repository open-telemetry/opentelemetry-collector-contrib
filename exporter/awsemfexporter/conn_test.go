// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"
)

var ec2Region = "us-west-2"

type mockConn struct {
	mock.Mock
	sn *session.Session
}

func (c *mockConn) getEC2Region(s *session.Session) (string, error) {
	args := c.Called(nil)
	errorStr := args.String(0)
	var err error
	if errorStr != "" {
		err = errors.New(errorStr)
		return "", err
	}
	return ec2Region, nil
}

func (c *mockConn) newAWSSession(logger *zap.Logger, roleArn string, region string) (*session.Session, error) {
	return c.sn, nil
}

// fetch region value from ec2 meta data service
func TestEC2Session(t *testing.T) {
	logger := zap.NewNop()
	emfExporterCfg := loadExporterConfig(t)
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, emfExporterCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, ec2Region, "Region value fetched from ec2-metadata service")
	assert.Nil(t, err)
}

// fetch region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	emfExporterCfg := loadExporterConfig(t)
	region := "us-east-1"
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_REGION", region)

	var m = &mockConn{}
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, emfExporterCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, region, "Region value fetched from environment")
	assert.Nil(t, err)
}

func TestGetAWSConfigSessionWithSessionErr(t *testing.T) {
	logger := zap.NewNop()
	emfExporterCfg := loadExporterConfig(t)
	emfExporterCfg.Region = ""
	emfExporterCfg.NoVerifySSL = false
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, emfExporterCfg)
	assert.Nil(t, cfg)
	assert.Nil(t, s)
	assert.NotNil(t, err)
}

func TestGetAWSConfigSessionWithEC2RegionErr(t *testing.T) {
	logger := zap.NewNop()
	emfExporterCfg := loadExporterConfig(t)
	emfExporterCfg.Region = ""
	emfExporterCfg.NoVerifySSL = false
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("some error").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, emfExporterCfg)
	assert.Nil(t, cfg)
	assert.Nil(t, s)
	assert.NotNil(t, err)
}

func TestNewAWSSessionWithErr(t *testing.T) {
	logger := zap.NewNop()
	roleArn := "fake_arn"
	region := "fake_region"
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	conn := &Conn{}
	se, err := conn.newAWSSession(logger, roleArn, region)
	assert.NotNil(t, err)
	assert.Nil(t, se)
	roleArn = ""
	se, err = conn.newAWSSession(logger, roleArn, region)
	assert.NotNil(t, err)
	assert.Nil(t, se)
	os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "regional")
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
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	_, err := getDefaultSession(logger)
	assert.NotNil(t, err)
}

func TestGetSTSCreds(t *testing.T) {
	logger := zap.NewNop()
	region := "fake_region"
	roleArn := ""
	_, err := getSTSCreds(logger, region, roleArn)
	assert.Nil(t, err)
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")
	_, err = getSTSCreds(logger, region, roleArn)
	assert.NotNil(t, err)
}

func loadExporterConfig(t *testing.T) *Config {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)
	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	otelcfg, _ := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	emfExporterCfg := otelcfg.Exporters["awsemf"].(*Config)
	return emfExporterCfg
}

func stashEnv() []string {
	env := os.Environ()
	os.Clearenv()

	return env
}

func popEnv(env []string) {
	os.Clearenv()

	for _, e := range env {
		p := strings.SplitN(e, "=", 2)
		os.Setenv(p[0], p[1])
	}
}
