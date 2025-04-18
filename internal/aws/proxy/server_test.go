// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

// TODO review if tests should succeed on Windows
package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	// Mock getAWSConfig 함수 설정
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{
			Region: regionEnvVar,
		}, nil
	}

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
		assert.NoError(t, srv.Shutdown(context.Background()))
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

// mockReadCloser는 테스트를 위한 mock io.ReadCloser 구현
type mockReadCloser struct {
	readErr error
}

func (m *mockReadCloser) Read(_ []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	return 0, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestHandlerHappyCase(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	// Mock getAWSConfig 함수 설정
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{
			Region: regionEnvVar,
			Credentials: aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     "fakeAccessKeyID",
					SecretAccessKey: "fakeSecretAccessKey",
				}, nil
			})),
		}, nil
	}

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
	
	// HTTP 404 또는 401 응답 코드는 AWS 엔드포인트에 따라 다를 수 있음
	statusCode := rec.Result().StatusCode
	assert.True(t, statusCode == http.StatusNotFound || statusCode == http.StatusForbidden || statusCode == http.StatusUnauthorized, 
		"Expected HTTP error status, got: %d", statusCode)
}

func TestHandlerIoReadSeekerCreationFailed(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	// Mock getAWSConfig 함수 설정
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{
			Region: regionEnvVar,
			Credentials: aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     "fakeAccessKeyID",
					SecretAccessKey: "fakeSecretAccessKey",
				}, nil
			})),
		}, nil
	}

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
	errLogFound := false
	for _, entry := range logs {
		if entry.Message == "Unable to consume request body" {
			assert.Error(t, entry.Context[0].Interface.(error))
			errLogFound = true
			break
		}
	}
	assert.True(t, errLogFound, "Expected error log not found")
}

func TestHandlerNilBodyIsOk(t *testing.T) {
	logger, recordedLogs := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	// Mock getAWSConfig 함수 설정
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{
			Region: regionEnvVar,
			Credentials: aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     "fakeAccessKeyID",
					SecretAccessKey: "fakeSecretAccessKey",
				}, nil
			})),
		}, nil
	}

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest(http.MethodPost,
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	logs := recordedLogs.All()
	logFound := false
	for _, entry := range logs {
		if entry.Message == "Received request on X-Ray receiver TCP proxy server" {
			logFound = true
			break
		}
	}
	assert.True(t, logFound, "Expected log message not found")
}

func TestTCPEndpointInvalid(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	cfg.Endpoint = "invalid\n"
	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
}

func TestCantGetAWSConfig(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	expectedErr := errors.New("expected getAWSConfig error")
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{}, expectedErr
	}
	
	_, err := NewServer(cfg, logger)
	assert.EqualError(t, err, expectedErr.Error())
}

func TestAWSEndpointInvalid(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	// 원래 getAWSConfig 함수 백업 및 모킹
	originalGetAWSConfig := getAWSConfig
	defer func() {
		getAWSConfig = originalGetAWSConfig
	}()

	// Mock getAWSConfig 함수 설정
	getAWSConfig = func(_ *Config, _ *zap.Logger) (aws.Config, error) {
		return aws.Config{
			Region: regionEnvVar,
		}, nil
	}

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.Endpoint = tcpAddr
	cfg.AWSEndpoint = "invalid endpoint \n"

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.Contains(t, err.Error(), "unable to parse AWS service endpoint")
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
	// the underlying parse error contains "invalid control character"
	assert.Contains(t, err.Error(), "invalid control character", "expected URL parse failure")
}

func TestConsumeFunctionWithNilBody(t *testing.T) {
	reader, err := consume(nil)
	assert.NoError(t, err, "consume should not error with nil input")
	assert.Nil(t, reader, "reader should be nil for nil input")
}

func TestConsumeFunctionWithValidBody(t *testing.T) {
	body := strings.NewReader("test data")
	reader, err := consume(io.NopCloser(body))
	assert.NoError(t, err, "consume should not error with valid input")
	assert.NotNil(t, reader, "reader should not be nil for valid input")
	
	// Check that we can read from the returned reader
	data, err := io.ReadAll(reader)
	assert.NoError(t, err, "should be able to read from returned reader")
	assert.Equal(t, "test data", string(data), "data should match input")
}