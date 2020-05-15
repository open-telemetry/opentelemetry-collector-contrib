// Copyright 2019, OpenTelemetry Authors
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

package awsxrayexporter

import (
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

var ec2Region = "us-east-1"

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
	xrayExporterCfg := loadExporterConfig(t)
	m := new(mockConn)
	m.On("getEC2Region", nil).Return("").Once()
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, xrayExporterCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, ec2Region, "Region value fetched from ec2-metadata service")
	assert.Nil(t, err)
}

// fetch region value from environment variable
func TestRegionEnv(t *testing.T) {
	logger := zap.NewNop()
	xrayExporterCfg := loadExporterConfig(t)
	region := "us-west-2"
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_REGION", region)

	var m = &mockConn{}
	var expectedSession *session.Session
	expectedSession, _ = session.NewSession()
	m.sn = expectedSession
	cfg, s, err := GetAWSConfigSession(logger, m, xrayExporterCfg)
	assert.Equal(t, s, expectedSession, "Expect the session object is not overridden")
	assert.Equal(t, *cfg.Region, region, "Region value fetched from environment")
	assert.Nil(t, err)
}

func loadExporterConfig(t *testing.T) *Config {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)
	factory := &Factory{}
	factories.Exporters[factory.Type()] = factory
	otelcfg, _ := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	xrayExporterCfg := otelcfg.Exporters["awsxray"].(*Config)
	return xrayExporterCfg
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
