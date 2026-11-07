// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestClientLoggerBodySettings(t *testing.T) {
	tests := []struct {
		name             string
		logRequestBody   bool
		logResponseBody  bool
		wantRequestBody  bool
		wantResponseBody bool
	}{
		{
			name: "disabled by default",
		},
		{
			name:             "request body enabled",
			logRequestBody:   true,
			wantRequestBody:  true,
			wantResponseBody: false,
		},
		{
			name:             "response body enabled",
			logResponseBody:  true,
			wantRequestBody:  false,
			wantResponseBody: true,
		},
		{
			name:             "both enabled",
			logRequestBody:   true,
			logResponseBody:  true,
			wantRequestBody:  true,
			wantResponseBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newClientLogger(
				zap.NewNop(),
				tt.logRequestBody,
				tt.logResponseBody,
			)

			require.Equal(t, tt.wantRequestBody, logger.RequestBodyEnabled())
			require.Equal(t, tt.wantResponseBody, logger.ResponseBodyEnabled())
		})
	}
}

func TestClientLoggerLogRoundTrip(t *testing.T) {
	tests := []struct {
		name             string
		logRequestBody   bool
		logResponseBody  bool
		requestFailed    bool
		wantRequestBody  bool
		wantResponseBody bool
		wantMessage      string
	}{
		{
			name:        "both body logs disabled",
			wantMessage: "Request roundtrip completed.",
		},
		{
			name:            "request body only",
			logRequestBody:  true,
			wantRequestBody: true,
			wantMessage:     "Request roundtrip completed.",
		},
		{
			name:             "response body only",
			logResponseBody:  true,
			wantResponseBody: true,
			wantMessage:      "Request roundtrip completed.",
		},
		{
			name:             "both body logs enabled",
			logRequestBody:   true,
			logResponseBody:  true,
			wantRequestBody:  true,
			wantResponseBody: true,
			wantMessage:      "Request roundtrip completed.",
		},
		{
			name:            "request body logged when request fails",
			logRequestBody:  true,
			requestFailed:   true,
			wantRequestBody: true,
			wantMessage:     "Request failed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			logger := newClientLogger(
				zap.New(core),
				tt.logRequestBody,
				tt.logResponseBody,
			)

			req, err := http.NewRequest(
				http.MethodPost,
				"http://localhost:9200/test",
				strings.NewReader(`{"foo":"bar"}`),
			)
			require.NoError(t, err)

			var resp *http.Response
			var roundTripErr error

			if tt.requestFailed {
				roundTripErr = errors.New("connection refused")
			} else {
				resp = &http.Response{
					Status:     "200 OK",
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"acknowledged":true}`)),
				}
			}

			err = logger.LogRoundTrip(req, resp, roundTripErr, time.Now(), time.Second)
			require.NoError(t, err)
			require.Equal(t, 1, logs.Len())

			entry := logs.All()[0]
			require.Equal(t, tt.wantMessage, entry.Message)

			fields := entry.ContextMap()

			requestBody, hasRequestBody := fields["request_body"]
			require.Equal(t, tt.wantRequestBody, hasRequestBody)
			if tt.wantRequestBody {
				require.Equal(t, `{"foo":"bar"}`, requestBody)
			}

			responseBody, hasResponseBody := fields["response_body"]
			require.Equal(t, tt.wantResponseBody, hasResponseBody)
			if tt.wantResponseBody {
				require.Equal(t, `{"acknowledged":true}`, responseBody)
			}
		})
	}
}

func TestWarnAboutBodyLogging(t *testing.T) {
	tests := []struct {
		name            string
		logRequestBody  bool
		logResponseBody bool
		wantWarning     bool
	}{
		{
			name:        "disabled",
			wantWarning: false,
		},
		{
			name:           "request body enabled",
			logRequestBody: true,
			wantWarning:    true,
		},
		{
			name:            "response body enabled",
			logResponseBody: true,
			wantWarning:     true,
		},
		{
			name:            "both enabled",
			logRequestBody:  true,
			logResponseBody: true,
			wantWarning:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			logger := zap.New(core)

			warnAboutBodyLogging(logger, TelemetrySettings{
				LogRequestBody:  tt.logRequestBody,
				LogResponseBody: tt.logResponseBody,
			})

			if tt.wantWarning {
				require.Equal(t, 1, logs.Len())
				require.Equal(
					t,
					"OpenSearch request/response body logging is enabled. Request and response bodies may contain sensitive information and should only be enabled for testing and debugging.",
					logs.All()[0].Message,
				)
			} else {
				require.Equal(t, 0, logs.Len())
			}
		})
	}
}
