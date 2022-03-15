// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sigv4authextension

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewSigv4Extension(t *testing.T) {
	cfg := &Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn"}}

	sa := newSigv4Extension(cfg, "awsSDKInfo", zap.NewNop())
	assert.Equal(t, cfg.Region, sa.cfg.Region)
	assert.Equal(t, cfg.Service, sa.cfg.Service)
	assert.Equal(t, cfg.AssumeRole.ARN, sa.cfg.AssumeRole.ARN)
}

func TestRoundTripper(t *testing.T) {
	awsCredsProvider := mockCredentials()

	base := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	awsSDKInfo := "awsSDKInfo"
	cfg := &Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn"}, credsProvider: awsCredsProvider}

	sa := newSigv4Extension(cfg, awsSDKInfo, zap.NewNop())
	assert.NotNil(t, sa)

	rt, err := sa.RoundTripper(base)
	assert.Nil(t, err)

	si := rt.(*signingRoundTripper)
	assert.Equal(t, base, si.transport)
	assert.Equal(t, cfg.Region, si.region)
	assert.Equal(t, cfg.Service, si.service)
	assert.Equal(t, awsSDKInfo, si.awsSDKInfo)
	assert.Equal(t, cfg.credsProvider, si.credsProvider)

}

func TestPerRPCCredentials(t *testing.T) {
	cfg := &Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn"}}
	sa := newSigv4Extension(cfg, "", zap.NewNop())

	rpc, err := sa.PerRPCCredentials()
	assert.Nil(t, rpc)
	assert.Error(t, err)
}

func TestGetCredsProviderFromConfig(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *Config
		AccessKeyID     string
		SecretAccessKey string
		shouldError     bool
	}{
		{
			"success_case_without_role",
			&Config{Region: "region", Service: "service"},
			"AccessKeyID",
			"SecretAccessKey",
			false,
		},
		{
			"failure_case_without_role",
			&Config{Region: "region", Service: "service"},
			"",
			"",
			true,
		},
	}
	// run tests
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY_ID", testcase.AccessKeyID)
			t.Setenv("AWS_SECRET_ACCESS_KEY", testcase.SecretAccessKey)
			credsProvider, err := getCredsProviderFromConfig(testcase.cfg)

			if testcase.shouldError {
				assert.Error(t, err)
				assert.Nil(t, credsProvider)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, credsProvider)

			creds, err := (*credsProvider).Retrieve(context.Background())
			require.NoError(t, err)
			require.NotNil(t, creds)
		})
	}
}

func TestCloneRequest(t *testing.T) {
	req1, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)

	req2, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)
	req2.Header.Add("Header1", "val1")

	tests := []struct {
		name    string
		request *http.Request
	}{
		{
			"no_headers",
			req1,
		},
		{
			"headers",
			req2,
		},
	}
	// run tests
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			r2 := cloneRequest(testcase.request)
			assert.EqualValues(t, testcase.request.Header, r2.Header)
			assert.EqualValues(t, testcase.request.Body, r2.Body)
		})
	}
}

func mockCredentials() *aws.CredentialsProvider {
	awscfg, _ := awsconfig.LoadDefaultConfig(context.Background())
	provider := credentials.NewStaticCredentialsProvider(
		"MOCK_AWS_ACCESS_KEY",
		"MOCK_AWS_SECRET_ACCESS_KEY",
		"MOCK_TOKEN",
	)

	awscfg.Credentials = aws.NewCredentialsCache(provider)

	return &awscfg.Credentials
}
