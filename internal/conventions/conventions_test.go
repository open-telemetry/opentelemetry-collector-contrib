// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package conventions_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/conventions"
)

// The purpose of the tests in this file is to highlight changes in semconv upstream

func TestStableConventions(t *testing.T) {
	require.Equal(t, "service.name", string(conventions.ServiceNameKey))
}

func TestDevelopmentConventions(t *testing.T) {
	require.Equal(t, "host.name", string(conventions.HostNameKey))
	require.Equal(t, "service.instance.id", string(conventions.ServiceInstanceIDKey))
}
