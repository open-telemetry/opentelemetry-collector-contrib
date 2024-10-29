// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !windows

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func testListener(t *testing.T) (net.Listener, string) {
	f, err := os.CreateTemp(os.TempDir(), "testListener")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	require.NoError(t, os.Remove(addr))

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}
