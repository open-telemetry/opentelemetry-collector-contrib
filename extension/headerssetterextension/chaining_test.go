// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

// mockAuthExtension is a mock auth extension for testing
type mockAuthExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*mockAuthExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &mockAuthRoundTripper{base: base, headerKey: "Authorization", headerValue: "Bearer token123"}, nil
}

func (*mockAuthExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &mockPerRPCCredentials{metadata: map[string]string{"authorization": "Bearer token123"}}, nil
}

// mockAuthRoundTripper adds mock auth headers
type mockAuthRoundTripper struct {
	base        http.RoundTripper
	headerKey   string
	headerValue string
}

func (m *mockAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set(m.headerKey, m.headerValue)
	return m.base.RoundTrip(req2)
}

// mockPerRPCCredentials provides mock gRPC credentials
type mockPerRPCCredentials struct {
	metadata map[string]string
}

func (m *mockPerRPCCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return m.metadata, nil
}

func (*mockPerRPCCredentials) RequireTransportSecurity() bool {
	return false
}

// mockHost provides a mock component.Host for testing
type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

func TestChainingWithAdditionalAuth_HTTP(t *testing.T) {
	// Create auth extension ID
	authID := component.MustNewIDWithName("mock_auth", "test")

	// Create headerssetter config with additional_auth
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:    stringp("X-Custom-Header"),
				Value:  stringp("custom-value"),
				Action: UPSERT,
			},
		},
		AdditionalAuth: &authID,
	}

	// Create headerssetter extension
	ext, err := newHeadersSetterExtension(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Create mock host with mock auth extension
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			authID: &mockAuthExtension{},
		},
	}

	// Start the extension
	err = ext.Start(t.Context(), host)
	require.NoError(t, err)

	// Get the RoundTripper
	captureRT := &requestCaptureRoundTripper{}
	rt, err := ext.RoundTripper(captureRT)
	require.NoError(t, err)
	require.NotNil(t, rt)

	// Create a test request
	req, err := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	require.NoError(t, err)

	// Execute the round trip
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	// Verify both auth headers are present in the captured request
	require.NotNil(t, captureRT.capturedRequest, "Request should be captured")
	assert.Equal(t, "Bearer token123", captureRT.capturedRequest.Header.Get("Authorization"), "OAuth2 header should be present")
	assert.Equal(t, "custom-value", captureRT.capturedRequest.Header.Get("X-Custom-Header"), "Custom header should be present")
}

func TestChainingWithAdditionalAuth_gRPC(t *testing.T) {
	// Create auth extension ID
	authID := component.MustNewIDWithName("mock_auth", "test")

	// Create headerssetter config with additional_auth
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:    stringp("x-custom-header"),
				Value:  stringp("custom-value"),
				Action: UPSERT,
			},
		},
		AdditionalAuth: &authID,
	}

	// Create headerssetter extension
	ext, err := newHeadersSetterExtension(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Create mock host with mock auth extension
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			authID: &mockAuthExtension{},
		},
	}

	// Start the extension
	err = ext.Start(t.Context(), host)
	require.NoError(t, err)

	// Get the PerRPCCredentials
	creds, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	require.NotNil(t, creds)

	// Get metadata
	metadata, err := creds.GetRequestMetadata(t.Context())
	require.NoError(t, err)

	// Verify both auth metadata are present
	assert.Equal(t, "Bearer token123", metadata["authorization"], "OAuth2 metadata should be present")
	assert.Equal(t, "custom-value", metadata["x-custom-header"], "Custom metadata should be present")
}

func TestChainingWithMissingAuth(t *testing.T) {
	// Create auth extension ID that doesn't exist
	authID := component.MustNewIDWithName("missing_auth", "test")

	// Create headerssetter config with additional_auth
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:    stringp("X-Custom-Header"),
				Value:  stringp("custom-value"),
				Action: UPSERT,
			},
		},
		AdditionalAuth: &authID,
	}

	// Create headerssetter extension
	ext, err := newHeadersSetterExtension(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Create mock host with NO auth extension
	host := &mockHost{
		Host:       componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{},
	}

	// Start the extension
	err = ext.Start(t.Context(), host)
	require.NoError(t, err)

	// Get the RoundTripper - should fail
	captureRT := &requestCaptureRoundTripper{}
	_, err = ext.RoundTripper(captureRT)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth extension")
	assert.Contains(t, err.Error(), "not found")
}

func TestWithoutAdditionalAuth(t *testing.T) {
	// Create headerssetter config WITHOUT additional_auth
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:    stringp("X-Custom-Header"),
				Value:  stringp("custom-value"),
				Action: UPSERT,
			},
		},
		AdditionalAuth: nil,
	}

	// Create headerssetter extension
	ext, err := newHeadersSetterExtension(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, ext)

	// Start should not be called when AdditionalAuth is nil
	assert.Nil(t, ext.StartFunc)

	// Get the RoundTripper - should work without starting
	captureRT := &requestCaptureRoundTripper{}
	rt, err := ext.RoundTripper(captureRT)
	require.NoError(t, err)
	require.NotNil(t, rt)

	// Create a test request
	req, err := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	require.NoError(t, err)

	// Execute the round trip
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	// Verify only custom header is present in captured request
	require.NotNil(t, captureRT.capturedRequest, "Request should be captured")
	assert.Empty(t, captureRT.capturedRequest.Header.Get("Authorization"), "Auth header should not be present")
	assert.Equal(t, "custom-value", captureRT.capturedRequest.Header.Get("X-Custom-Header"), "Custom header should be present")
}

func TestDependencies(t *testing.T) {
	authID := component.MustNewIDWithName("oauth2", "test")

	tests := []struct {
		name           string
		additionalAuth *component.ID
		expectedDeps   []component.ID
	}{
		{
			name:           "with additional auth",
			additionalAuth: &authID,
			expectedDeps:   []component.ID{authID},
		},
		{
			name:           "without additional auth",
			additionalAuth: nil,
			expectedDeps:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:    stringp("X-Test"),
						Value:  stringp("test"),
						Action: UPSERT,
					},
				},
				AdditionalAuth: tt.additionalAuth,
			}

			ext, err := newHeadersSetterExtension(cfg, zap.NewNop())
			require.NoError(t, err)

			deps := ext.Dependencies()
			assert.Equal(t, tt.expectedDeps, deps)
		})
	}
}

// requestCaptureRoundTripper captures the request for testing
type requestCaptureRoundTripper struct {
	capturedRequest *http.Request
}

func (m *requestCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture the request for verification
	m.capturedRequest = req
	// Just return a dummy response
	return &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
	}, nil
}

// Ensure interface compliance
var (
	_ extensionauth.HTTPClient = (*mockAuthExtension)(nil)
	_ extensionauth.GRPCClient = (*mockAuthExtension)(nil)
	_ component.Host           = (*mockHost)(nil)
)
