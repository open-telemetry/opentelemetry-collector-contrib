// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

// TODO review if tests should succeed on Windows
package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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
	cfg.TCPAddr.Endpoint = tcpAddr
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

func TestHandlerHappyCase(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.TCPAddr.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest("POST",
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
	cfg.TCPAddr.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	expectedErr := errors.New("expected mockReadCloser error")
	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest("POST",
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
	cfg.TCPAddr.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest("POST",
		"https://xray.us-west-2.amazonaws.com/GetSamplingRules", nil)
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
	cfg.TCPAddr.Endpoint = tcpAddr
	srv, err := NewServer(cfg, logger)
	assert.NoError(t, err, "NewServer should succeed")

	handler := srv.(*http.Server).Handler.ServeHTTP
	req := httptest.NewRequest("POST",
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
	cfg.TCPAddr.Endpoint = "invalid\n"
	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
}

func TestCantGetAWSConfigSession(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.TCPAddr.Endpoint = tcpAddr

	real := newAWSSession
	defer func() {
		newAWSSession = real
	}()

	expectedErr := errors.New("expected newAWSSessionError")
	newAWSSession = func(roleArn string, region string, log *zap.Logger) (*session.Session, error) {
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
	cfg.TCPAddr.Endpoint = tcpAddr

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.Contains(t, err.Error(), "invalid region")
}

func TestAWSEndpointInvalid(t *testing.T) {
	logger, _ := logSetup()

	t.Setenv(regionEnvVarName, regionEnvVar)

	cfg := DefaultConfig()
	tcpAddr := testutil.GetAvailableLocalAddress(t)
	cfg.TCPAddr.Endpoint = tcpAddr
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
	cfg.TCPAddr.Endpoint = tcpAddr
	cfg.ProxyAddress = "invalid address \n"

	_, err := NewServer(cfg, logger)
	assert.Error(t, err, "NewServer should fail")
	assert.Contains(t, err.Error(), "failed to parse proxy URL")
}

func TestGetServiceEndpointInvalidAWSConfig(t *testing.T) {
	_, err := getServiceEndpoint(&aws.Config{})
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

func (m *mockReadCloser) Close() error {
	return nil
}
