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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
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

	credErr := errors.New("mock credential retrieval error")
	origNewAWSConfig := newAWSConfig
	newAWSConfig = func(ctx context.Context, roleArn, region string, log *zap.Logger) (aws.Config, error) {
		cfg, err := origNewAWSConfig(ctx, roleArn, region, log)
		if err != nil {
			return cfg, err
		}
		cfg.Credentials = aws.CredentialsProviderFunc(func(_ context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, credErr
		})
		return cfg, nil
	}
	t.Cleanup(func() { newAWSConfig = origNewAWSConfig })

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
	assert.EqualError(t, lastEntry.Context[0].Interface.(error),
		credErr.Error(), "expected error")
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

	endpoint, err = getServiceEndpoint("us-gov-west-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.us-gov-west-1.amazonaws.com", endpoint)

	endpoint, err = getServiceEndpoint("us-iso-east-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.us-iso-east-1.c2s.ic.gov", endpoint)

	endpoint, err = getServiceEndpoint("us-isob-east-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.us-isob-east-1.sc2s.sgov.gov", endpoint)

	endpoint, err = getServiceEndpoint("eu-isoe-west-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.eu-isoe-west-1.cloud.adc-e.uk", endpoint)

	endpoint, err = getServiceEndpoint("us-isof-south-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.us-isof-south-1.csp.hci.ic.gov", endpoint)

	endpoint, err = getServiceEndpoint("eusc-de-east-1", "xray")
	assert.NoError(t, err)
	assert.Equal(t, "https://xray.eusc-de-east-1.amazonaws.eu", endpoint)
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
	capturedReqCh := make(chan *http.Request, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReqCh <- r.Clone(r.Context())
	}))
	defer backend.Close()

	// Override the transport to use mock backend
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy

	// Save original rewrite and wrap it
	originalRewrite := proxy.Rewrite
	proxy.Rewrite = func(r *httputil.ProxyRequest) {
		originalRewrite(r)
		// Redirect to mock backend
		r.Out.URL.Scheme = "http"
		r.Out.URL.Host = backend.Listener.Addr().String()
		r.Out.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{"NextToken": null}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	// Verify Authorization header format
	var capturedReq *http.Request
	select {
	case capturedReq = <-capturedReqCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for backend to receive request")
	}
	require.NotNil(t, capturedReq, "backend should have received the request")
	authHeader := capturedReq.Header.Get("Authorization")
	require.NotEmpty(t, authHeader, "Authorization header should be present")
	// AWS SigV4 Authorization header format:
	// AWS4-HMAC-SHA256 Credential=.../aws4_request, SignedHeaders=..., Signature=...
	assert.Contains(t, authHeader, "AWS4-HMAC-SHA256", "should use AWS4-HMAC-SHA256 algorithm")
	assert.Contains(t, authHeader, "Credential=", "should contain Credential")
	assert.Contains(t, authHeader, "SignedHeaders=", "should contain SignedHeaders")
	assert.Contains(t, authHeader, "Signature=", "should contain Signature")
	assert.Contains(t, authHeader, "aws4_request", "should contain aws4_request scope terminator")
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
	capturedReqCh := make(chan *http.Request, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReqCh <- r.Clone(r.Context())
	}))
	defer backend.Close()

	// Override the transport to use mock backend
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy

	originalRewrite := proxy.Rewrite
	proxy.Rewrite = func(r *httputil.ProxyRequest) {
		originalRewrite(r)
		r.Out.URL.Scheme = "http"
		r.Out.URL.Host = backend.Listener.Addr().String()
		r.Out.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	var capturedReq *http.Request
	select {
	case capturedReq = <-capturedReqCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for backend to receive request")
	}
	require.NotNil(t, capturedReq, "backend should have received the request")
	// X-Amz-Date header is required for SigV4
	xAmzDate := capturedReq.Header.Get("X-Amz-Date")
	require.NotEmpty(t, xAmzDate, "X-Amz-Date header should be present")
	// Format: 20060102T150405Z (ISO 8601 basic format)
	assert.Regexp(t, `^\d{8}T\d{6}Z$`, xAmzDate, "X-Amz-Date should be in ISO 8601 basic format")
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

	capturedReqCh := make(chan *http.Request, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReqCh <- r.Clone(r.Context())
	}))
	defer backend.Close()

	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy

	originalRewrite := proxy.Rewrite
	proxy.Rewrite = func(r *httputil.ProxyRequest) {
		originalRewrite(r)
		r.Out.URL.Scheme = "http"
		r.Out.URL.Host = backend.Listener.Addr().String()
		r.Out.Host = backend.Listener.Addr().String()
	}

	handler := httpSrv.Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules",
		strings.NewReader(`{}`))
	// Add Connection header that should be removed before signing
	req.Header.Set("Connection", "keep-alive")
	rec := httptest.NewRecorder()
	handler(rec, req)

	var capturedReq *http.Request
	select {
	case capturedReq = <-capturedReqCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for backend to receive request")
	}
	require.NotNil(t, capturedReq, "backend should have received the request")
	authHeader := capturedReq.Header.Get("Authorization")
	require.NotEmpty(t, authHeader, "Authorization header should be present")
	// Connection header should NOT be in SignedHeaders
	assert.NotContains(t, strings.ToLower(authHeader), "connection",
		"Connection header should not be signed")
}

func setupTestEnv(t *testing.T) (*zap.Logger, *observer.ObservedLogs) {
	t.Helper()
	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")
	return logSetup()
}

type mockTransport struct {
	capturedRequests []*http.Request
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.capturedRequests = append(m.capturedRequests, req)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func TestBuildRoutingMapsEmpty(t *testing.T) {
	apiMap, credsByRole := buildRoutingMaps(context.Background(), nil, "", nil, "", &awsutil.AWSSessionSettings{}, zap.NewNop())
	assert.Empty(t, apiMap)
	assert.Empty(t, credsByRole)
}

func TestBuildRoutingMapsValid(t *testing.T) {
	logger, _ := logSetup()

	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents", "CreateLogGroup"},
			ServiceName: "logs",
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
		{
			Paths:       []string{"PutTraceSegments"},
			ServiceName: "xray",
			AWSEndpoint: "https://xray.us-west-2.amazonaws.com",
		},
	}

	apiMap, credsByRole := buildRoutingMaps(context.Background(), routes, "", nil, "us-west-2", &awsutil.AWSSessionSettings{}, logger)
	assert.Len(t, apiMap, 3)
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName)
	assert.Equal(t, "logs", apiMap["CreateLogGroup"].ServiceName)
	assert.Equal(t, "xray", apiMap["PutTraceSegments"].ServiceName)
	assert.Empty(t, credsByRole)
}

func TestBuildRoutingMapsInvalidRules(t *testing.T) {
	logger, _ := setupTestEnv(t)

	routes := []RoutingRule{
		// Missing service_name
		{
			Paths:       []string{"MissingServiceName"},
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
		// Valid rule
		{
			Paths:       []string{"ValidRule"},
			ServiceName: "xray",
			AWSEndpoint: "https://xray.us-west-2.amazonaws.com",
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "us-west-2", &awsutil.AWSSessionSettings{}, logger)

	// Invalid rule (missing service_name) is mapped to nil
	assert.Nil(t, apiMap["MissingServiceName"], "missing service_name should map to nil")

	// Valid rule is mapped correctly
	assert.NotNil(t, apiMap["ValidRule"])
	assert.Equal(t, "xray", apiMap["ValidRule"].ServiceName)
}

func TestInvalidRoutingRuleSkipsSigning(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths: []string{"InvalidPath"},
			// Missing service_name - invalid rule
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err)

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	handler := httpSrv.Handler.(*proxyHandler)
	handler.proxy.Transport = mockTrans

	req := httptest.NewRequest(http.MethodPost, "http://localhost:2000/InvalidPath", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// proxyHandler should reject the request with 400 before it reaches the reverse proxy
	assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid route should be rejected")
	assert.Empty(t, mockTrans.capturedRequests, "request should not reach the reverse proxy")
}

func TestBuildRoutingMapsMissingEndpoint(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			Region:      "us-east-1",
			// AWSEndpoint will be resolved from service name and region
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "", &awsutil.AWSSessionSettings{}, zap.NewNop())
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName)
}

func TestBuildRoutingMapsResolvesEndpointAtStartup(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			Region:      "us-east-1",
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "", &awsutil.AWSSessionSettings{}, zap.NewNop())
	assert.Equal(t, "https://logs.us-east-1.amazonaws.com", apiMap["PutLogEvents"].AWSEndpoint, "endpoint should be resolved at startup")
}

func TestBuildRoutingMapsFallsBackToDefaultRegion(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			// No Region - should fall back to defaultRegion
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "us-west-2", &awsutil.AWSSessionSettings{}, zap.NewNop())
	assert.NotNil(t, apiMap["PutLogEvents"])
	assert.Equal(t, "us-west-2", apiMap["PutLogEvents"].Region, "should fall back to default region")
}

func TestBuildRoutingMapsAutoResolvesEndpoint(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			Region:      "us-east-1",
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "", &awsutil.AWSSessionSettings{}, zap.NewNop())
	assert.Equal(t, "https://logs.us-east-1.amazonaws.com", apiMap["PutLogEvents"].AWSEndpoint, "should auto-resolve endpoint from service_name and region")
}

func TestNewServerWithRoutingRules(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed with routing rules")
	assert.NotNil(t, srv)
}

func TestBuildRoutingMapsWithLeadingSlash(t *testing.T) {
	logger, _ := logSetup()

	routes := []RoutingRule{
		{
			Paths:       []string{"/PutLogEvents", "CreateLogGroup"},
			ServiceName: "logs",
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "us-west-2", &awsutil.AWSSessionSettings{}, logger)
	assert.Len(t, apiMap, 2)
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName)
	assert.Equal(t, "logs", apiMap["CreateLogGroup"].ServiceName)
}

func TestBuildRoutingMapsDuplicateAPIs(t *testing.T) {
	logger, _ := logSetup()

	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "xray",
			AWSEndpoint: "https://xray.us-west-2.amazonaws.com",
		},
	}

	apiMap, _ := buildRoutingMaps(context.Background(), routes, "", nil, "us-west-2", &awsutil.AWSSessionSettings{}, logger)
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName, "first route should win")
}

func TestHandlerRoutingWithMultipleServices(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents", "CreateLogGroup"},
			ServiceName: "logs",
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
			Region:      "us-east-1",
		},
		{
			Paths:       []string{"PutTraceSegments"},
			ServiceName: "xray",
			AWSEndpoint: "https://xray.us-west-2.amazonaws.com",
			Region:      "us-west-2",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	// Replace transport with mock
	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
	}{
		{"/PutLogEvents", "logs.us-east-1.amazonaws.com"},
		{"/CreateLogGroup", "logs.us-east-1.amazonaws.com"},
		{"/PutTraceSegments", "xray.us-west-2.amazonaws.com"},
		{"/UnmatchedAPI", "xray.us-west-2.amazonaws.com"}, // fallback to default
	}

	for _, tc := range testCases {
		mockTrans.capturedRequests = nil
		req := httptest.NewRequest(http.MethodPost, "https://example.com"+tc.apiPath, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)

		assert.Len(t, mockTrans.capturedRequests, 1, "should have captured one request for %s", tc.apiPath)
		capturedReq := mockTrans.capturedRequests[0]
		assert.Equal(t, tc.expectedHost, capturedReq.Host, "API %s should route to %s", tc.apiPath, tc.expectedHost)
	}
}

func TestHandlerRoutingWithAutoResolvedEndpoint(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents", "CreateLogGroup"},
			ServiceName: "logs",
			Region:      "us-east-1",
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			Region:      "eu-west-1",
		},
		{
			Paths:       []string{"PutServiceLevelObjective"},
			ServiceName: "applicationsignals",
			Region:      "ap-south-1",
		},
		{
			Paths:       []string{"SendMessage"},
			ServiceName: "sqs",
			Region:      "us-west-2",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
	}{
		{"/PutLogEvents", "logs.us-east-1.amazonaws.com"},
		{"/CreateLogGroup", "logs.us-east-1.amazonaws.com"},
		{"/PutMetricData", "monitoring.eu-west-1.amazonaws.com"},
		{"/PutServiceLevelObjective", "applicationsignals.ap-south-1.amazonaws.com"},
		{"/SendMessage", "sqs.us-west-2.amazonaws.com"},
	}

	for _, tc := range testCases {
		mockTrans.capturedRequests = nil
		req := httptest.NewRequest(http.MethodPost, "http://localhost:2000"+tc.apiPath, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)

		assert.Len(t, mockTrans.capturedRequests, 1, "should have captured one request for %s", tc.apiPath)
		capturedReq := mockTrans.capturedRequests[0]
		assert.Equal(t, tc.expectedHost, capturedReq.Host, "API %s should auto-resolve to %s", tc.apiPath, tc.expectedHost)
	}
}

func TestHandlerPreservesURLPath(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"slos"},
			ServiceName: "application-signals",
			Region:      "us-east-1",
			AWSEndpoint: "https://application-signals.us-east-1.api.aws",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy
	proxy.Transport = mockTrans

	req := httptest.NewRequest(http.MethodPost, "http://localhost:2000/slos?MaxResults=1", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Len(t, mockTrans.capturedRequests, 1)
	capturedReq := mockTrans.capturedRequests[0]
	assert.Equal(t, "application-signals.us-east-1.api.aws", capturedReq.Host)
	assert.Equal(t, "/slos", capturedReq.URL.Path, "URL path should be preserved")
	assert.Equal(t, "MaxResults=1", capturedReq.URL.RawQuery, "query string should be preserved")
}

func TestNewServerWithMultipleRoles(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.RoleARN = "arn:aws:iam::123456789012:role/DefaultRole"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			Region:      "us-east-1",
			RoleARN:     "arn:aws:iam::123456789012:role/LogsRole",
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			Region:      "us-west-2",
			RoleARN:     "arn:aws:iam::123456789012:role/MetricsRole",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed with multiple roles")
	assert.NotNil(t, srv)
}

func TestNewServerWithDuplicateRoles(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.RoleARN = "arn:aws:iam::123456789012:role/SharedRole"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			RoleARN:     "arn:aws:iam::123456789012:role/SharedRole",
		},
		{
			Paths:       []string{"CreateLogGroup"},
			ServiceName: "logs",
			RoleARN:     "arn:aws:iam::123456789012:role/SharedRole",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed with duplicate roles")
	assert.NotNil(t, srv)
}

func TestNewServerWithDefaultRoleInRoutingRules(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.RoleARN = "arn:aws:iam::123456789012:role/DefaultRole"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			RoleARN:     "arn:aws:iam::123456789012:role/DefaultRole",
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			RoleARN:     "arn:aws:iam::123456789012:role/OtherRole",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed when routing rule references default role")
	assert.NotNil(t, srv)
}

func TestNewServerWithEmptyRoleARN(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			RoleARN:     "",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed with empty role ARN in routing rule")
	assert.NotNil(t, srv)
}

func TestNewServerWithMixedRoleConfiguration(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.RoleARN = "arn:aws:iam::123456789012:role/DefaultRole"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			RoleARN:     "arn:aws:iam::123456789012:role/LogsRole",
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			// No RoleARN - should use default
		},
		{
			Paths:       []string{"PutTraceSegments"},
			ServiceName: "xray",
			RoleARN:     "",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed with mixed role configuration")
	assert.NotNil(t, srv)
}

func TestHandlerRoutingFallsBackToTopLevelRegion(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.ServiceName = "xray"
	cfg.Region = "eu-west-1" // Top-level region
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			// No Region - should fall back to top-level eu-west-1
			// No AWSEndpoint - should auto-resolve using logs + eu-west-1
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			Region:      "ap-south-1", // Explicit region
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
		description  string
	}{
		{"/PutLogEvents", "logs.eu-west-1.amazonaws.com", "rule without region falls back to top-level region"},
		{"/PutMetricData", "monitoring.ap-south-1.amazonaws.com", "rule with explicit region uses its own region"},
	}

	for _, tc := range testCases {
		mockTrans.capturedRequests = nil
		req := httptest.NewRequest(http.MethodPost, "http://localhost:2000"+tc.apiPath, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)

		assert.Len(t, mockTrans.capturedRequests, 1, "should have captured one request for %s", tc.apiPath)
		capturedReq := mockTrans.capturedRequests[0]
		assert.Equal(t, tc.expectedHost, capturedReq.Host, "%s: %s", tc.apiPath, tc.description)
	}
}

func TestHandlerRoutingAutoResolvesEndpointWhenRuleHasNoEndpoint(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.ServiceName = "application-signals"
	cfg.Region = "us-west-2"
	cfg.AWSEndpoint = "https://application-signals-gamma.us-west-2.api.aws"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"slos"},
			ServiceName: "application-signals",
			Region:      "us-west-2",
			AWSEndpoint: "https://application-signals-gamma.us-west-2.api.aws",
		},
		{
			Paths:       []string{"GetSamplingRules", "SamplingTargets"},
			ServiceName: "xray",
			Region:      "us-west-2",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*proxyHandler).proxy
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
		description  string
	}{
		{"/slos", "application-signals-gamma.us-west-2.api.aws", "rule with explicit endpoint"},
		{"/GetSamplingRules", "xray.us-west-2.amazonaws.com", "rule without endpoint auto-resolves from service_name and region"},
		{"/SamplingTargets", "xray.us-west-2.amazonaws.com", "rule without endpoint auto-resolves from service_name and region"},
	}

	for _, tc := range testCases {
		mockTrans.capturedRequests = nil
		req := httptest.NewRequest(http.MethodPost, "http://localhost:2000"+tc.apiPath, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, req)

		assert.Len(t, mockTrans.capturedRequests, 1, "should have captured one request for %s", tc.apiPath)
		capturedReq := mockTrans.capturedRequests[0]
		assert.Equal(t, tc.expectedHost, capturedReq.Host, "%s: %s", tc.apiPath, tc.description)
	}
}
