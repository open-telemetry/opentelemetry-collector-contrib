// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
)

func testListener(t *testing.T) (net.Listener, string) {
	addr := "\\\\.\\pipe\\testListener-otel-collector-contrib"

	l, err := winio.ListenPipe(addr, nil)
	require.NoError(t, err)
	require.NotNil(t, l)
	return l, addr
}
