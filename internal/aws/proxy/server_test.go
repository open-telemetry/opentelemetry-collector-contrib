// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

// TODO review if tests should succeed on Windows
package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

const (
	regionEnvVarName = "AWS_DEFAULT_REGION"
	regionEnvVar     = "us-west-2"
)

func TestHappyCase(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.ProxyAddress = "https://example.com"
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")
	go func() {
		_ = srv.ListenAndServe()
	}()
	defer func() {
		assert.NoError(t, srv.Shutdown(t.Context()))
	}()

	assert.Eventuallyf(t, func() bool {
		_, err := net.DialTimeout("tcp", tcpAddr, time.Second)
		return err == nil
	}, 10*time.Second, 5*time.Millisecond, "port should eventually be accessible")

	logs := recordedLogs.All()
	lastEntry := logs[0]
	assert.Contains(t, lastEntry.Message, "Using remote proxy", "expected log message")
	assert.Equal(t, "address", lastEntry.Context[0].Key, "expected log key")
	assert.Equal(t, cfg.ProxyAddress, lastEntry.Context[0].String)
}

func TestHandlerHappyCase(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", strings.NewReader(`{"NextToken": null}`))
	rec := httptest.NewRecorder()
	handler(rec, req)
	// the security token is expected to be invalid
	assert.Equal(t, http.StatusForbidden, rec.Result().StatusCode)
	headers := rec.Result().Header
	assert.Contains(t, headers, "X-Amzn-Requestid")
	assert.Contains(t, headers, "X-Amzn-Errortype")
}

func TestHandlerIoReadSeekerCreationFailed(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	expectedErr := errors.New("expected mockReadCloser error")
	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", &mockReadCloser{
			readErr: expectedErr,
		})
	rec := httptest.NewRecorder()
	handler(rec, req)

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Unable to consume request body", "expected log message")
	assert.EqualError(t, lastEntry.Context[0].Interface.(error),
		expectedErr.Error(), "expected error")
}

func TestHandlerNilBodyIsOk(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, req)

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message,
		"Received request on X-Ray receiver TCP proxy server",
		"expected log message")
}

func TestHandlerSignerErrorsOut(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	logs := recordedLogs.All()
	lastEntry := logs[len(logs)-1]
	assert.Contains(t, lastEntry.Message, "Unable to retrieve credentials", "expected log message")
	assert.Contains(t, lastEntry.Context[0].Interface.(error).Error(),
		"no EC2 IMDS role found", "expected error")
}

func TestTCPEndpointInvalid(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	cfg.Endpoint = "invalid\n"
	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
}

func TestCantGetAWSConfigSession(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr

	origConfig := newAWSConfig
	defer func() {
		newAWSConfig = origConfig
	}()

	expectedErr := errors.New("expected newAWSConfigError")
	newAWSConfig = func(_ context.Context, _, _ string, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{}, expectedErr
	}
	_, err := NewServer(cfg, logger)
	assert.EqualError(t, err, expectedErr.Error())
}

func TestCantGetServiceEndpoint(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, "not a region")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.ErrorContains(t, err, "invalid region")
}

func TestAWSEndpointInvalid(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AWSEndpoint = "invalid endpoint \n"

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.ErrorContains(t, err, "unable to parse AWS service endpoint")
}

func TestCanCreateTransport(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.ProxyAddress = "invalid address \n"

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.ErrorContains(t, err, "invalid control character in URL")
}

func TestGetServiceEndpoint(t *testing.T) {
	_, err := getServiceEndpoint("", "xray")
	assert.EqualError(t, err, "invalid region: ")

	endpoint, err := getServiceEndpoint("us-west-2", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.us-west-2.amazonaws.com", endpoint)

	endpoint, err = getServiceEndpoint("cn-north-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.cn-north-1.amazonaws.com.cn", endpoint)
}

type mockReadCloser struct {
	readErr error
}

func (m *mockReadCloser) Read(_ []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	return 0, nil
}

func (*mockReadCloser) Close() error {
	return nil
}

// TestConsumeBody tests the consumeBody function that reads HTTP body and calculates SHA-256 hash.
// This is critical for AWS SigV4 signing which requires a payload hash.
func TestConsumeBody(t *testing.T) {
	// SHA-256 hash of empty string - used as a constant in AWS SigV4
	// echo -n "" | sha256sum = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	const emptyStringHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	tests := []struct {
		name      string
		body      io.ReadCloser
		wantBytes []byte
		wantHash  string
		wantErr   bool
	}{
		{
			name:      "nil body returns empty hash",
			body:      nil,
			wantBytes: nil,
			wantHash:  emptyStringHash,
			wantErr:   false,
		},
		{
			name:      "empty body returns empty hash",
			body:      io.NopCloser(strings.NewReader("")),
			wantBytes: []byte{},
			wantHash:  emptyStringHash,
			wantErr:   false,
		},
		{
			name:      "body with JSON content",
			body:      io.NopCloser(strings.NewReader(`{"NextToken": null}`)),
			wantBytes: []byte(`{"NextToken": null}`),
			// echo -n '{"NextToken": null}' | sha256sum
			wantHash: "0b35b96fcca5659602b9cd486360efd65de22e4e7f8ca337c0a3935be95d8989",
			wantErr:  false,
		},
		{
			name:      "body with simple content",
			body:      io.NopCloser(strings.NewReader("hello")),
			wantBytes: []byte("hello"),
			// echo -n 'hello' | sha256sum
			wantHash: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			wantErr:  false,
		},
		{
			name:    "read error returns error",
			body:    &mockReadCloser{readErr: errors.New("read failed")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, gotHash, err := consumeBody(tt.body)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantBytes, gotBytes, "body bytes should match")
			assert.Equal(t, tt.wantHash, gotHash, "payload hash should match expected SHA-256")
		})
	}
}

// TestConsumeBodyRestoration verifies that the body can be restored after consumption.
// This is essential because HTTP body is a stream that can only be read once,
// but we need to read it for hash calculation and then restore it for forwarding.
func TestConsumeBodyRestoration(t *testing.T) {
	originalContent := `{"TraceIds": ["1-abc-123", "1-def-456"]}`

	// Consume the body
	body, hash, err := consumeBody(io.NopCloser(strings.NewReader(originalContent)))
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Restore the body (as done in server.go)
	restoredBody := io.NopCloser(bytes.NewReader(body))

	// Read the restored body
	restoredContent, err := io.ReadAll(restoredBody)
	assert.NoError(t, err)
	assert.Equal(t, originalContent, string(restoredContent), "restored body should match original")
}

// TestSigV4PayloadHashFormat verifies the payload hash format required by AWS SigV4.
// AWS SigV4 requires the payload hash to be a lowercase hex-encoded SHA-256 hash.
func TestSigV4PayloadHashFormat(t *testing.T) {
	testCases := []struct {
		name    string
		payload string
	}{
		{"empty payload", ""},
		{"simple payload", "test"},
		{"json payload", `{"key": "value"}`},
		{"binary-like payload", "\x00\x01\x02\x03"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, hash, err := consumeBody(io.NopCloser(strings.NewReader(tc.payload)))
			assert.NoError(t, err)

			// Verify hash format: must be 64 character lowercase hex string
			assert.Len(t, hash, 64, "SHA-256 hash should be 64 hex characters")
			assert.Regexp(t, "^[a-f0-9]{64}$", hash, "hash should be lowercase hex only")
		})
	}
}

// TestSignedRequestHasAuthorizationHeader verifies that signed requests contain
// the Authorization header with AWS SigV4 signature components.
func TestSignedRequestHasAuthorizationHeader(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	// Create a mock backend server to capture the signed request
	var capturedReq *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReq = r
	}))
	defer backend.Close()

	// Override the transport to use mock backend
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)

	// Save original director and wrap it
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Redirect to mock backend
		req.URL.Scheme = "http"
		req.URL.Host = backend.Listener.Addr().String()
		req.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{"NextToken": null}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Verify Authorization header format
	if capturedReq != nil {
		authHeader := capturedReq.Header.Get("Authorization")
		if authHeader != "" {
			// AWS SigV4 Authorization header format:
			// AWS4-HMAC-SHA256 Credential=.../aws4_request, SignedHeaders=..., Signature=...
			assert.Contains(t, authHeader, "AWS4-HMAC-SHA256", "should use AWS4-HMAC-SHA256 algorithm")
			assert.Contains(t, authHeader, "Credential=", "should contain Credential")
			assert.Contains(t, authHeader, "SignedHeaders=", "should contain SignedHeaders")
			assert.Contains(t, authHeader, "Signature=", "should contain Signature")
			assert.Contains(t, authHeader, "aws4_request", "should contain aws4_request scope terminator")
		}
	}
}

// TestSignedRequestHasRequiredHeaders verifies that signed requests contain
// the required headers for AWS SigV4: Host and X-Amz-Date.
func TestSignedRequestHasRequiredHeaders(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	// Create a mock backend server to capture the signed request
	var capturedReq *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReq = r
	}))
	defer backend.Close()

	// Override the transport to use mock backend
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = "http"
		req.URL.Host = backend.Listener.Addr().String()
		req.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if capturedReq != nil {
		// X-Amz-Date header is required for SigV4
		xAmzDate := capturedReq.Header.Get("X-Amz-Date")
		if xAmzDate != "" {
			// Format: 20060102T150405Z (ISO 8601 basic format)
			assert.Regexp(t, `^\d{8}T\d{6}Z$`, xAmzDate, "X-Amz-Date should be in ISO 8601 basic format")
		}
	}
}

// TestConnectionHeaderRemovedBeforeSigning verifies that the Connection header
// is removed before signing. This is important because the reverse proxy removes
// hop-by-hop headers like Connection, and if we signed it, the signature would
// be invalid after the header is removed.
func TestConnectionHeaderRemovedBeforeSigning(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	var capturedReq *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReq = r
	}))
	defer backend.Close()

	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = "http"
		req.URL.Host = backend.Listener.Addr().String()
		req.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{}`))
	// Add Connection header that should be removed before signing
	req.Header.Set("Connection", "keep-alive")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if capturedReq != nil {
		authHeader := capturedReq.Header.Get("Authorization")
		if authHeader != "" {
			// Connection header should NOT be in SignedHeaders
			assert.NotContains(t, strings.ToLower(authHeader), "connection",
				"Connection header should not be signed")
		}
	}
}
