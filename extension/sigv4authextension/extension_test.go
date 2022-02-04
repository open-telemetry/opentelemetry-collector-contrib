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
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewSigv4Extension(t *testing.T) {
	cfg := &Config{Region: "region", Service: "service", RoleArn: "rolearn"}

	sa := newSigv4Extension(cfg, "awsSDKInfo", zap.NewNop())
	assert.Equal(t, cfg.Region, sa.cfg.Region)
	assert.Equal(t, cfg.Service, sa.cfg.Service)
	assert.Equal(t, cfg.RoleArn, sa.cfg.RoleArn)
}

func TestRoundTripper(t *testing.T) {
	awsCreds := fetchMockCredentials()
	base := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	awsSDKInfo := "awsSDKInfo"
	cfg := &Config{Region: "region", Service: "service", RoleArn: "rolearn", creds: awsCreds}

	sa:= newSigv4Extension(cfg, awsSDKInfo, zap.NewNop())
	assert.NotNil(t, sa)

	rt, err := sa.RoundTripper(base)
	assert.Nil(t, err)

	si := rt.(*SigningRoundTripper)
	assert.Equal(t, base, si.transport)
	assert.Equal(t, cfg.Region, si.region)
	assert.Equal(t, cfg.Service, si.service)
	assert.Equal(t, awsSDKInfo, si.awsSDKInfo)

}

func TestPerRPCCredentials(t *testing.T) {
	cfg := &Config{Region: "region", Service: "service", RoleArn: "rolearn"}
	sa := newSigv4Extension(cfg, "", zap.NewNop())

	rpc, err := sa.PerRPCCredentials()
	assert.Nil(t, rpc)
	assert.Error(t, err)
}

func TestGetCredsFromConfig(t *testing.T) {
	awsCreds := fetchMockCredentials()
	t.Setenv("AWS_ACCESS_KEY_ID", awsCreds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", awsCreds.SecretAccessKey)
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			"success_case_without_role",
			&Config{Region: "region", Service: "service"},
		},
	}
	// run tests
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			creds, err := getCredsFromConfig(testcase.cfg)
			require.NoError(t, err, "Failed getCredsFromConfig")
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

func fetchMockCredentials() *aws.Credentials {
	provider := credentials.NewStaticCredentialsProvider(
		"MOCK_AWS_ACCESS_KEY",
		"MOCK_AWS_SECRET_ACCESS_KEY",
		"MOCK_TOKEN",
	)
	return &provider.Value
}
