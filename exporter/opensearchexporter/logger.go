// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opensearchexporter contains an opentelemetry-collector exporter
// for OpenSearch.
package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v2/opensearchtransport"
	"go.uber.org/zap"
)

type clientLogger struct {
	zapLogger *zap.Logger
}

func newClientLogger(zl *zap.Logger) opensearchtransport.Logger {
	return &clientLogger{zl}
}

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (cl *clientLogger) LogRoundTrip(requ *http.Request, resp *http.Response, err error, _ time.Time, dur time.Duration) error {
	switch {
	case err == nil && resp != nil:
		cl.zapLogger.Debug("Request roundtrip completed.",
			zap.String("path", requ.URL.Path),
			zap.String("method", requ.Method),
			zap.Duration("duration", dur),
			zap.String("status", resp.Status))

	case err != nil:
		cl.zapLogger.Error("Request failed.",
			zap.String("path", requ.URL.Path),
			zap.String("method", requ.Method),
			zap.Duration("duration", dur),
			zap.NamedError("reason", err))
	}

	return nil
}

// RequestBodyEnabled makes the client pass a copy of request body to the logger.
func (*clientLogger) RequestBodyEnabled() bool {
	// TODO: introduce setting log the bodies for more detailed debug logs
	return false
}

// ResponseBodyEnabled makes the client pass a copy of response body to the logger.
func (*clientLogger) ResponseBodyEnabled() bool {
	// TODO: introduce setting log the bodies for more detailed debug logs
	return false
}
