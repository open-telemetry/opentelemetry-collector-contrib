// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

// TODO review if tests should succeed on Windows
package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"
	"time"

	//nolint:staticcheck // SA1019: WIP in https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36699
	"github.com/aws/aws-sdk-go/aws"
	//nolint:staticcheck // SA1019: WIP in https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36699
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

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
	assert.Contains(t, lastEntry.Message, "Unable to sign request", "expected log message")
	assert.Contains(t, lastEntry.Context[0].Interface.(error).Error(),
		"NoCredentialProviders", "expected error")
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

	origSession := newAWSSession
	defer func() {
		newAWSSession = origSession
	}()

	expectedErr := errors.New("expected newAWSSessionError")
	newAWSSession = func(string, string, *zap.Logger) (*session.Session, error) {
		return nil, expectedErr
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
	assert.ErrorContains(t, err, "failed to parse proxy URL")
}

func TestGetServiceEndpointInvalidAWSConfig(t *testing.T) {
	_, err := getServiceEndpoint(&aws.Config{}, "")
	assert.EqualError(t, err, "unable to generate endpoint from region with nil value")
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
	apiMap, signerMap := buildRoutingMaps(nil, "", nil, "", &sessionConfig{}, nil)
	assert.Empty(t, apiMap)
	assert.Empty(t, signerMap)
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

	apiMap, signerMap := buildRoutingMaps(routes, "", nil, "us-west-2", &sessionConfig{}, logger)
	assert.Len(t, apiMap, 3)
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName)
	assert.Equal(t, "logs", apiMap["CreateLogGroup"].ServiceName)
	assert.Equal(t, "xray", apiMap["PutTraceSegments"].ServiceName)
	assert.Empty(t, signerMap)
}

func TestBuildRoutingMapsInvalidRules(t *testing.T) {
	logger, _ := setupTestEnv(t)

	routes := []RoutingRule{
		{
			Paths:       []string{"MissingServiceName"},
			AWSEndpoint: "https://logs.us-east-1.amazonaws.com",
		},
		{
			Paths:       []string{"ValidRule"},
			ServiceName: "xray",
			AWSEndpoint: "https://xray.us-west-2.amazonaws.com",
		},
	}

	apiMap, _ := buildRoutingMaps(routes, "", nil, "us-west-2", &sessionConfig{}, logger)
	assert.Nil(t, apiMap["MissingServiceName"], "missing service_name should map to nil")
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
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err)

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
	proxy.Transport = mockTrans

	req := httptest.NewRequest(http.MethodPost, "http://localhost:2000/InvalidPath", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	assert.Len(t, mockTrans.capturedRequests, 1)
	capturedReq := mockTrans.capturedRequests[0]
	assert.Empty(t, capturedReq.Header.Get("Authorization"), "invalid route should not be signed")
}

func TestBuildRoutingMapsResolvesEndpointAtStartup(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
			Region:      "us-east-1",
		},
	}

	apiMap, _ := buildRoutingMaps(routes, "", nil, "", &sessionConfig{}, nil)
	assert.Equal(t, "https://logs.us-east-1.amazonaws.com", apiMap["PutLogEvents"].AWSEndpoint, "endpoint should be resolved at startup")
}

func TestBuildRoutingMapsFallsBackToDefaultRegion(t *testing.T) {
	routes := []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
		},
	}

	apiMap, _ := buildRoutingMaps(routes, "", nil, "us-west-2", &sessionConfig{}, nil)
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

	apiMap, _ := buildRoutingMaps(routes, "", nil, "", &sessionConfig{}, nil)
	assert.Equal(t, "https://logs.us-east-1.amazonaws.com", apiMap["PutLogEvents"].AWSEndpoint, "should auto-resolve endpoint from service_name and region")
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

	apiMap, _ := buildRoutingMaps(routes, "", nil, "us-west-2", &sessionConfig{}, logger)
	assert.Equal(t, "logs", apiMap["PutLogEvents"].ServiceName, "first route should win")
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

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
	}{
		{"/PutLogEvents", "logs.us-east-1.amazonaws.com"},
		{"/CreateLogGroup", "logs.us-east-1.amazonaws.com"},
		{"/PutTraceSegments", "xray.us-west-2.amazonaws.com"},
		{"/UnmatchedAPI", "xray.us-west-2.amazonaws.com"},
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
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
	proxy.Transport = mockTrans

	testCases := []struct {
		apiPath      string
		expectedHost string
	}{
		{"/PutLogEvents", "logs.us-east-1.amazonaws.com"},
		{"/CreateLogGroup", "logs.us-east-1.amazonaws.com"},
		{"/PutMetricData", "monitoring.eu-west-1.amazonaws.com"},
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
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
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

func TestHandlerRoutingFallsBackToTopLevelRegion(t *testing.T) {
	logger, _ := setupTestEnv(t)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.ServiceName = "xray"
	cfg.Region = "eu-west-1"
	cfg.AdditionalRoutingRules = []RoutingRule{
		{
			Paths:       []string{"PutLogEvents"},
			ServiceName: "logs",
		},
		{
			Paths:       []string{"PutMetricData"},
			ServiceName: "monitoring",
			Region:      "ap-south-1",
		},
	}

	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	mockTrans := &mockTransport{}
	httpSrv := srv.(*http.Server)
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
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
	proxy := httpSrv.Handler.(*httputil.ReverseProxy)
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
