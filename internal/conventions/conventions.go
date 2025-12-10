// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package conventions // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/conventions"

import semconv "go.opentelemetry.io/otel/semconv/v1.38.0"

const SchemaURL = semconv.SchemaURL

// stable as of v1.38.0
const (
	ExceptionMessageKey    = semconv.ExceptionMessageKey
	ExceptionStacktraceKey = semconv.ExceptionStacktraceKey
	ExceptionTypeKey       = semconv.ExceptionTypeKey
	ServiceNameKey         = semconv.ServiceNameKey
	ServiceVersionKey      = semconv.ServiceVersionKey
)

// development as of v1.38.0
const (
	HostNameKey          = semconv.HostNameKey
	HostArchKey          = semconv.HostArchKey
	OSTypeKey            = semconv.OSTypeKey
	ServiceInstanceIDKey = semconv.ServiceInstanceIDKey
	ServiceNamespaceKey  = semconv.ServiceNamespaceKey
)
