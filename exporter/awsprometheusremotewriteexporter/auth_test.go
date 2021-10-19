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

package awsprometheusremotewriteexporter

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

var sdkInformation = fmt.Sprintf("%s/%s (%s; %s; %s)", aws.SDKName, aws.SDKVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH)

func TestRequestSignature(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	awsCreds := fetchMockCredentials()
	authConfig := AuthConfig{Region: "region", Service: "service"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := v4.GetSignedRequestSignature(r)
		assert.NoError(t, err)
		assert.Equal(t, sdkInformation, r.Header.Get("User-Agent"))
		w.WriteHeader(200)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	setting := confighttp.HTTPClientSettings{
		Endpoint:        serverURL.String(),
		TLSSetting:      configtls.TLSClientSetting{},
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		Timeout:         0,
		CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
			return newSigningRoundTripperWithCredentials(authConfig, awsCreds, next, sdkInformation)
		},
	}
	client, _ := setting.ToClient(componenttest.NewNopHost().GetExtensions())
	req, err := http.NewRequest("POST", setting.Endpoint, strings.NewReader("a=1&b=2"))
	assert.NoError(t, err)
	_, err = client.Do(req)
	assert.NoError(t, err)
}

type checkCloser struct {
	io.Reader
	mu     sync.Mutex
	closed bool
}

func (cc *checkCloser) Close() error {
	cc.mu.Lock()
	cc.closed = true
	cc.mu.Unlock()
	return nil
}

func TestLeakingBody(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	awsCreds := fetchMockCredentials()
	authConfig := AuthConfig{Region: "region", Service: "service"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := v4.GetSignedRequestSignature(r)
		assert.NoError(t, err)
		assert.Equal(t, sdkInformation, r.Header.Get("User-Agent"))
		w.WriteHeader(200)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	setting := confighttp.HTTPClientSettings{
		Endpoint:        serverURL.String(),
		TLSSetting:      configtls.TLSClientSetting{},
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		Timeout:         0,
		CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
			return newSigningRoundTripperWithCredentials(authConfig, awsCreds, next, sdkInformation)
		},
	}
	client, _ := setting.ToClient(componenttest.NewNopHost().GetExtensions())
	checker := &checkCloser{Reader: strings.NewReader("a=1&b=2")}
	req, err := http.NewRequest("POST", setting.Endpoint, checker)
	assert.NoError(t, err)
	req.GetBody = func() (io.ReadCloser, error) {
		checker.Reader = strings.NewReader("a=1&b=2")
		return checker, nil
	}
	_, err = client.Do(req)
	assert.NoError(t, err)
	assert.True(t, checker.closed)
}

func TestGetCredsFromConfig(t *testing.T) {
	tests := []struct {
		name       string
		authConfig AuthConfig
	}{
		{
			"success_case_without_role",
			AuthConfig{Region: "region", Service: "service"},
		},
		{
			"success_case_with_role",
			AuthConfig{Region: "region", Service: "service", RoleArn: "arn:aws:iam::123456789012:role/IAMRole"},
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := getCredsFromConfig(tt.authConfig)
			require.NoError(t, err, "Failed getCredsFromConfig")
			require.NotNil(t, creds)
		})
	}
}

type ErrorRoundTripper struct{}

func (ert *ErrorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errors.New("error")
}

func TestRoundTrip(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	awsCreds := fetchMockCredentials()

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	errorRoundTripper := &ErrorRoundTripper{}

	tests := []struct {
		name        string
		rt          http.RoundTripper
		shouldError bool
		authConfig  AuthConfig
	}{
		{
			"valid_round_tripper",
			defaultRoundTripper,
			false,
			AuthConfig{Region: "region", Service: "service"},
		},
		{
			"round_tripper_error",
			errorRoundTripper,
			true,
			AuthConfig{Region: "region", Service: "service", RoleArn: "arn:aws:iam::123456789012:role/IAMRole"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := v4.GetSignedRequestSignature(r)
				assert.NoError(t, err)
				assert.Equal(t, sdkInformation, r.Header.Get("User-Agent"))
				w.WriteHeader(200)
			}))
			defer server.Close()
			serverURL, _ := url.Parse(server.URL)
			authConfig := AuthConfig{Region: "region", Service: "service"}
			rt, err := newSigningRoundTripperWithCredentials(authConfig, awsCreds, tt.rt, sdkInformation)
			assert.NoError(t, err)
			req, err := http.NewRequest("POST", serverURL.String(), strings.NewReader(""))
			assert.NoError(t, err)
			res, err := rt.RoundTrip(req)
			if tt.shouldError {
				assert.Nil(t, res)
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, res.StatusCode, 200)
		})
	}
}

func TestCreateSigningRoundTripperWithCredentials(t *testing.T) {
	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())

	// Some form of AWS credentials must be set up for tests to succeed
	awsCreds := fetchMockCredentials()

	tests := []struct {
		name         string
		creds        *credentials.Credentials
		roundTripper http.RoundTripper
		authConfig   AuthConfig
		authApplied  bool
		returnError  bool
	}{
		{
			"success_case",
			awsCreds,
			defaultRoundTripper,
			AuthConfig{Region: "region", Service: "service"},
			true,
			false,
		},
		{
			"success_case_no_auth_applied",
			awsCreds,
			defaultRoundTripper,
			AuthConfig{Region: "", Service: ""},
			false,
			false,
		},
		{
			"no_credentials_provided_error",
			nil,
			defaultRoundTripper,
			AuthConfig{Region: "region", Service: "service"},
			true,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rtp, err := newSigningRoundTripperWithCredentials(tt.authConfig, tt.creds, tt.roundTripper, sdkInformation)
			if tt.returnError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.authApplied {
				sRtp := rtp.(*signingRoundTripper)
				assert.Equal(t, sRtp.transport, tt.roundTripper)
				assert.Equal(t, tt.authConfig.Service, sRtp.service)
			}
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
		headers http.Header
	}{
		{
			"no_headers",
			req1,
			http.Header{},
		},
		{
			"headers",
			req2,
			http.Header{"Header1": []string{"val1"}},
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r2 := cloneRequest(tt.request)
			assert.EqualValues(t, tt.request.Header, r2.Header)
		})
	}
}

func TestParseEndpointRegion(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantRegion string
		wantErr    bool
	}{
		{
			name:     "empty",
			endpoint: "",
			wantErr:  true,
		},
		{
			name:     "invalid",
			endpoint: "https://aps-workspaces.us-east-1",
			wantErr:  true,
		},
		{
			name:       "valid",
			endpoint:   "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write",
			wantRegion: "us-east-1",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRegion, err := parseEndpointRegion(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEndpointRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRegion != tt.wantRegion {
				t.Errorf("parseEndpointRegion() = %v, want %v", gotRegion, tt.wantRegion)
			}
		})
	}
}

func fetchMockCredentials() *credentials.Credentials {
	return credentials.NewStaticCredentials(
		"MOCK_AWS_ACCESS_KEY",
		"MOCK_AWS_SECRET_ACCESS_KEY",
		"MOCK_TOKEN",
	)
}
