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

// Package awsprometheusremotewriteexporter provides a Prometheus Remote Write Exporter with AWS Sigv4 authentication
package awsprometheusremotewriteexporter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestRequestSignature(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := v4.GetSignedRequestSignature(r)
		assert.NoError(t, err)
		w.WriteHeader(200)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	setting := confighttp.HTTPClientSettings{
		Endpoint:        serverURL.String(),
		TLSSetting:      configtls.TLSClientSetting{},
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		Timeout:         0,
		CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
			settings := AuthConfig{Region: "region", Service: "service"}
			return newSigningRoundTripper(settings, next)
		},
	}
	client, _ := setting.ToClient()
	req, err := http.NewRequest("POST", setting.Endpoint, strings.NewReader("a=1&b=2"))
	assert.NoError(t, err)
	client.Do(req)

}

type ErrorRoundTripper struct{}

func (ert *ErrorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errors.New("error")
}

func TestRoundTrip(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	errorRoundTripper := &ErrorRoundTripper{}

	tests := []struct {
		name        string
		rt          http.RoundTripper
		shouldError bool
	}{
		{
			"valid_round_tripper",
			defaultRoundTripper,
			false,
		},
		{
			"round_tripper_error",
			errorRoundTripper,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := v4.GetSignedRequestSignature(r)
				assert.NoError(t, err)
				w.WriteHeader(200)
			}))
			defer server.Close()
			serverURL, _ := url.Parse(server.URL)
			settings := AuthConfig{Region: "region", Service: "service"}
			rt, err := newSigningRoundTripper(settings, tt.rt)
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

func TestNewSigningRoundTripper(t *testing.T) {

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())

	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	tests := []struct {
		name         string
		roundTripper http.RoundTripper
		settings     AuthConfig
		authApplied  bool
		returnError  bool
	}{
		{
			"success_case",
			defaultRoundTripper,
			AuthConfig{Region: "region", Service: "service"},
			true,
			false,
		},
		{
			"success_case_no_auth_applied",
			defaultRoundTripper,
			AuthConfig{Region: "", Service: ""},
			false,
			false,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rtp, err := newSigningRoundTripper(tt.settings, tt.roundTripper)
			if tt.returnError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.authApplied {
				sRtp := rtp.(*signingRoundTripper)
				assert.Equal(t, sRtp.transport, tt.roundTripper)
				assert.Equal(t, tt.settings.Service, sRtp.service)
			} else {
				assert.Equal(t, rtp, tt.roundTripper)
			}
		})
	}
}

func TestCloneRequest(t *testing.T) {
	req1, err := http.NewRequest("GET", "http://example.com", nil)
	assert.NoError(t, err)

	req2, err := http.NewRequest("GET", "http://example.com", nil)
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
