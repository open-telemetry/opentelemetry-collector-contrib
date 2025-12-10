// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package conventions // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/conventions"

import semconv "go.opentelemetry.io/otel/semconv/v1.38.0"

// stable as of v1.38.0
const (
	ServiceNameKey       = semconv.ServiceNameKey
	ServiceInstanceIDKey = semconv.ServiceInstanceIDKey
)

// development as of v1.38.0
const (
	HostNameKey = semconv.HostNameKey
)
