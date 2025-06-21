// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/oauth2"
)

type mockRoundTripper struct {
	mock.Mock
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestStsTokenSource(t *testing.T) {
	// Create a temporary KSA token file
	mockKsaTokenFile, err := os.CreateTemp("", "test_ksa_token_path")
	assert.NoError(t, err)
	defer os.Remove(mockKsaTokenFile.Name())
	_, err = mockKsaTokenFile.WriteString("test-ksa-token")
	assert.NoError(t, err)
	err = mockKsaTokenFile.Close()
	assert.NoError(t, err)

	testCases := []struct {
		name          string
		ksaTokenPath  string
		mockSetup     func(*mockRoundTripper)
		expectedToken *oauth2.Token
		expectedError string
	}{
		{
			name:         "Successful token exchange",
			ksaTokenPath: mockKsaTokenFile.Name(),
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(bytes.NewBufferString(`{
                        "access_token": "test_sts_token",
                        "expires_in": 3600
                    }`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedToken: &oauth2.Token{
				AccessToken: "test_sts_token",
				Expiry:      time.Now().Add(3600*time.Second - jitterTime),
			},
		},
		{
			name:          "Failed to read security token",
			ksaTokenPath:  "non_existent_path",
			mockSetup:     func(m *mockRoundTripper) {},
			expectedError: "failed to read security token from file",
		},
		{
			name:         "STS request failure",
			ksaTokenPath: mockKsaTokenFile.Name(),
			mockSetup: func(m *mockRoundTripper) {
				m.On("RoundTrip", mock.Anything).Return((*http.Response)(nil), fmt.Errorf("network error"))
			},
			expectedError: "failed to exchange token: Post \"test_sts_url\": network error",
		},
		{
			name:         "Invalid JSON response",
			ksaTokenPath: mockKsaTokenFile.Name(),
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`invalid json`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "failed to parse STS Server json response",
		},
		{
			name:         "Missing access_token in response",
			ksaTokenPath: mockKsaTokenFile.Name(),
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"expires_in": 3600}`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "access_token not found",
		},
		{
			name:         "Missing expires_in in response",
			ksaTokenPath: mockKsaTokenFile.Name(),
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"access_token": "test_token"}`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "expires_in not found in JSON response",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRT := new(mockRoundTripper)
			tc.mockSetup(mockRT)

			config := &Config{
				SubjectTokenFile: tc.ksaTokenPath,
				TokenURL:         "test_sts_url",
				Audience:         "test_access_audience",
			}

			ts, _ := newStsTokenSource(config, mockRT)

			token, err := ts.Token()

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedToken.AccessToken, token.AccessToken)
				assert.WithinDuration(t, tc.expectedToken.Expiry, token.Expiry, time.Second)
			}

			mockRT.AssertExpectations(t)
		})
	}
}
