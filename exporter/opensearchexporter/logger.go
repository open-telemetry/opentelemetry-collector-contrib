// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opensearchexporter contains an opentelemetry-collector exporter
// for OpenSearch.
package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"io"
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchtransport"
	"go.uber.org/zap"
)

type clientLogger struct {
	zapLogger       *zap.Logger
	logRequestBody  bool
	logResponseBody bool
}

func newClientLogger(zl *zap.Logger, logRequestBody, logResponseBody bool) opensearchtransport.Logger {
	return &clientLogger{
		zapLogger:       zl,
		logRequestBody:  logRequestBody,
		logResponseBody: logResponseBody,
	}
}

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.

func (cl *clientLogger) LogRoundTrip(
	requ *http.Request,
	resp *http.Response,
	err error,
	_ time.Time,
	dur time.Duration,
) error {
	var fields []zap.Field

	if requ != nil {
		if requ.URL != nil {
			fields = append(fields, zap.String("path", requ.URL.Path))
		}
		fields = append(fields, zap.String("method", requ.Method))
	}

	fields = append(fields, zap.Duration("duration", dur))

	// Log the request body on both successful and failed round trips.
	if cl.logRequestBody && requ != nil && requ.Body != nil && requ.Body != http.NoBody {
		if body, readErr := io.ReadAll(requ.Body); readErr == nil {
			fields = append(fields, zap.ByteString("request_body", body))
		}
	}

	// Log a response body whenever a response is available.
	if cl.logResponseBody && resp != nil && resp.Body != nil && resp.Body != http.NoBody {
		if body, readErr := io.ReadAll(resp.Body); readErr == nil {
			fields = append(fields, zap.ByteString("response_body", body))
		}
	}

	switch {
	case err == nil && resp != nil:
		fields = append(fields, zap.String("status", resp.Status))
		cl.zapLogger.Debug("Request roundtrip completed.", fields...)
	case err != nil:
		fields = append(fields, zap.NamedError("reason", err))
		cl.zapLogger.Error("Request failed.", fields...)
	}

	return nil
}

// RequestBodyEnabled makes the client pass a copy of request body to the logger.
func (cl *clientLogger) RequestBodyEnabled() bool {
	return cl.logRequestBody
}

// ResponseBodyEnabled makes the client pass a copy of response body to the logger.
func (cl *clientLogger) ResponseBodyEnabled() bool {
	return cl.logResponseBody
}

func warnAboutBodyLogging(logger *zap.Logger, settings TelemetrySettings) {
	if settings.LogRequestBody || settings.LogResponseBody {
		logger.Warn(
			"OpenSearch request/response body logging is enabled. Request and response bodies may contain sensitive information and should only be enabled for testing and debugging.",
		)
	}
}
