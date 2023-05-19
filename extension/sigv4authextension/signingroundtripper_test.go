// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type errorRoundTripper struct{}

func (ert *errorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errors.New("error")
}

func TestRoundTrip(t *testing.T) {
	awsCredsProvider := mockCredentials()

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())
	errorRoundTripper := &errorRoundTripper{}

	tests := []struct {
		name        string
		rt          http.RoundTripper
		shouldError bool
		cfg         *Config
	}{
		{
			"valid_round_tripper",
			defaultRoundTripper,
			false,
			&Config{Region: "region", Service: "service", credsProvider: awsCredsProvider},
		},
		{
			"error_round_tripper",
			errorRoundTripper,
			true,
			&Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn", STSRegion: "region"}, credsProvider: awsCredsProvider},
		},
		{
			"error_invalid_credsProvider",
			defaultRoundTripper,
			true,
			&Config{Region: "region", Service: "service", credsProvider: nil},
		},
	}

	awsSDKInfo := "awsSDKInfo"
	body := "body"

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, awsSDKInfo, r.Header.Get("User-Agent"))

				reqBody := r.Body
				content, err := io.ReadAll(reqBody)

				assert.NoError(t, err)
				assert.Equal(t, body, string(content))

				w.WriteHeader(200)
			}))
			defer server.Close()
			serverURL, _ := url.Parse(server.URL)

			sa := newSigv4Extension(testcase.cfg, awsSDKInfo, zap.NewNop())
			rt, err := sa.RoundTripper(testcase.rt)
			assert.NoError(t, err)

			newBody := strings.NewReader(body)
			req, err := http.NewRequest("POST", serverURL.String(), newBody)
			assert.NoError(t, err)

			res, err := rt.RoundTrip(req)
			if testcase.shouldError {
				assert.Nil(t, res)
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, res.StatusCode, 200)
		})
	}
}

func TestInferServiceAndRegion(t *testing.T) {
	req1, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)

	req2, err := http.NewRequest("GET", "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write", nil)
	assert.NoError(t, err)

	req3, err := http.NewRequest("GET", "https://search-my-domain.us-east-1.es.amazonaws.com/_search?q=house", nil)
	assert.NoError(t, err)

	req4, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)

	req5, err := http.NewRequest("GET", "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write", nil)
	assert.NoError(t, err)

	tests := []struct {
		name            string
		request         *http.Request
		cfg             *Config
		expectedService string
		expectedRegion  string
	}{
		{
			"no_service_or_region_match_with_no_config",
			req1,
			createDefaultConfig().(*Config),
			"",
			"",
		},
		{
			"amp_service_and_region_match_with_no_config",
			req2,
			createDefaultConfig().(*Config),
			"aps",
			"us-east-1",
		},
		{
			"es_service_and_region_match_with_no_config",
			req3,
			createDefaultConfig().(*Config),
			"es",
			"us-east-1",
		},
		{
			"no_match_with_config",
			req4,
			&Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn", STSRegion: "region"}},
			"service",
			"region",
		},
		{
			"match_with_config",
			req5,
			&Config{Region: "region", Service: "service", AssumeRole: AssumeRole{ARN: "rolearn", STSRegion: "region"}},
			"service",
			"region",
		},
	}

	// run tests
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			sa := newSigv4Extension(testcase.cfg, "awsSDKInfo", zap.NewNop())
			assert.NotNil(t, sa)

			rt, err := sa.RoundTripper((http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone()))
			assert.Nil(t, err)
			si := rt.(*signingRoundTripper)

			service, region := si.inferServiceAndRegion(testcase.request)
			assert.EqualValues(t, testcase.expectedService, service)
			assert.EqualValues(t, testcase.expectedRegion, region)
		})
	}
}

func TestHashPayload(t *testing.T) {
	req1, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)

	req2, err := http.NewRequest("GET", "https://example.com", bytes.NewReader([]byte("This is a test.")))
	assert.NoError(t, err)

	req3, err := http.NewRequest("GET", "https://example.com", nil)
	assert.NoError(t, err)
	req3.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("this will always fail") }

	tests := map[string]struct {
		request      *http.Request
		expectedHash string
		expectError  bool
	}{
		"empty_body": {
			request:      req1,
			expectedHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		"simple_body": {
			request:      req2,
			expectedHash: "a8a2f6ebe286697c527eb35a58b5539532e9b3ae3b64d4eb0a46fb657b41562c",
		},
		"GetBody_error": {
			request:     req3,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res, err := hashPayload(tc.request)
			assert.Equal(t, tc.expectedHash, res)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
