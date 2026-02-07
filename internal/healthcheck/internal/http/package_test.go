// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/http"

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// These goroutines are part of the http.Client's connection pool management.
	// They don't accept context.Context and are managed by the transport's lifecycle,
	// not our test lifecycle. They'll be cleaned up when the transport is garbage collected.
	opts := []goleak.Option{
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyTestMain(m, opts...)
}
