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
	"github.com/stretchr/testify/require"
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

func TestHandlerWithFakeCredentials(t *testing.T) {
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
	assert.Contains(t, lastEntry.Message, "unable to retrieve credentials", "expected log message")
	assert.Error(t, lastEntry.Context[0].Interface.(error), "expected error")
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
	newAWSConfig = func(string, string, *zap.Logger) (aws.Config, error) {
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
	assert.ErrorContains(t, err, "unable to parse AWS service endpoint")
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
	_, err := getServiceEndpoint(aws.Config{}, "xray")
	assert.EqualError(t, err, "unable to generate endpoint from region with empty value")
}

func TestCalculatePayloadHash(t *testing.T) {
	tests := []struct {
		name string
		body io.Reader
		want string
	}{
		{
			name: "nil body",
			body: nil,
			want: emptyPayloadHash,
		},
		{
			name: "empty string body",
			body: strings.NewReader(""),
			want: emptyPayloadHash,
		},
		{
			name: "simple JSON payload",
			body: strings.NewReader(`{"key": "value"}`),
			want: "9724c1e20e6e3e4d7f57ed25f9d4efb006e508590d528c90da597f6a775c13e5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body == nil {
				req = &http.Request{Body: nil}
			} else {
				req = &http.Request{Body: io.NopCloser(tt.body)}
			}

			got, err := calculatePayloadHash(req)

			require.NoError(t, err)
			assert.NotEmpty(t, got, "hash should not be empty")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetServiceEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		config      aws.Config
		serviceName string
		want        string
	}{
		{
			name: "uses BaseEndpoint when provided",
			config: aws.Config{
				Region:       "us-east-1",
				BaseEndpoint: aws.String("https://custom.endpoint.com"),
			},
			serviceName: "xray",
			want:        "https://custom.endpoint.com",
		},
		{
			name: "ignores empty BaseEndpoint",
			config: aws.Config{
				Region:       "us-west-2",
				BaseEndpoint: aws.String(""),
			},
			serviceName: "s3",
			want:        "https://s3.us-west-2.amazonaws.com",
		},
		{
			name: "builds standard AWS region endpoint",
			config: aws.Config{
				Region: "eu-west-1",
			},
			serviceName: "dynamodb",
			want:        "https://dynamodb.eu-west-1.amazonaws.com",
		},
		{
			name: "builds China region endpoint",
			config: aws.Config{
				Region: "cn-north-1",
			},
			serviceName: "lambda",
			want:        "https://lambda.cn-north-1.amazonaws.com.cn",
		},
		{
			name: "builds GovCloud region endpoint",
			config: aws.Config{
				Region: "us-gov-west-1",
			},
			serviceName: "ec2",
			want:        "https://ec2.us-gov-west-1.amazonaws.com",
		},
		{
			name: "handles custom service names",
			config: aws.Config{
				Region: "ap-southeast-2",
			},
			serviceName: "application-autoscaling",
			want:        "https://application-autoscaling.ap-southeast-2.amazonaws.com",
		},
		{
			name: "BaseEndpoint takes priority over region",
			config: aws.Config{
				Region:       "us-east-1",
				BaseEndpoint: aws.String("http://localhost:8080"),
			},
			serviceName: "xray",
			want:        "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getServiceEndpoint(tt.config, tt.serviceName)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetServiceEndpoint_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		config      aws.Config
		serviceName string
		wantErr     string
	}{
		{
			name: "empty region returns error",
			config: aws.Config{
				Region: "",
			},
			serviceName: "xray",
			wantErr:     "unable to generate endpoint from region with empty value",
		},
		{
			name: "nil BaseEndpoint with empty region fails",
			config: aws.Config{
				Region:       "",
				BaseEndpoint: nil,
			},
			serviceName: "s3",
			wantErr:     "unable to generate endpoint from region with empty value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getServiceEndpoint(tt.config, tt.serviceName)

			require.Error(t, err)
			assert.Empty(t, got)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestBuildServiceEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		serviceName string
		want        string
	}{
		{
			name:        "standard AWS region",
			region:      "us-east-1",
			serviceName: "xray",
			want:        "https://xray.us-east-1.amazonaws.com",
		},
		{
			name:        "AWS GovCloud region",
			region:      "us-gov-west-1",
			serviceName: "s3",
			want:        "https://s3.us-gov-west-1.amazonaws.com",
		},
		{
			name:        "AWS China region",
			region:      "cn-north-1",
			serviceName: "dynamodb",
			want:        "https://dynamodb.cn-north-1.amazonaws.com.cn",
		},
		{
			name:        "unknown region defaults to standard format",
			region:      "unknown-region-1",
			serviceName: "customservice",
			want:        "https://customservice.unknown-region-1.amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildServiceEndpoint(tt.region, tt.serviceName)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
