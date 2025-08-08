// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	mockKsaTokenFile := filepath.Join(t.TempDir(), "test_ksa_token_path")
	err := os.WriteFile(mockKsaTokenFile, []byte("test-ksa-token"), 0644)
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
			ksaTokenPath: mockKsaTokenFile,
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
			ksaTokenPath: mockKsaTokenFile,
			mockSetup: func(m *mockRoundTripper) {
				m.On("RoundTrip", mock.Anything).Return((*http.Response)(nil), fmt.Errorf("network error"))
			},
			expectedError: "failed to exchange token: Post \"test_sts_url\": network error",
		},
		{
			name:         "Invalid JSON response",
			ksaTokenPath: mockKsaTokenFile,
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`invalid json`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "failed to decode token from response",
		},
		{
			name:         "Missing access_token in response",
			ksaTokenPath: mockKsaTokenFile,
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"expires_in": 3600}`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "access_token missing",
		},
		{
			name:         "Expired token",
			ksaTokenPath: mockKsaTokenFile,
			mockSetup: func(m *mockRoundTripper) {
				response := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"access_token": "test_token", "expires_in": 1}`)),
				}
				m.On("RoundTrip", mock.Anything).Return(response, nil)
			},
			expectedError: "token expired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRT := new(mockRoundTripper)
			tc.mockSetup(mockRT)

			config := &Config{
				SubjectTokenFile: tc.ksaTokenPath,
				AuthMode:         "sts",
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
