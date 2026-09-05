// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver

import (
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

// TestCloseWithNilConnDoesNotPanic guards against the SIGSEGV reported in
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49703:
// when a request times out mid-scrape, the connection-reset path in client.go
// calls Close then Connect; if the re-dial fails, gosnmp's netConnect leaves
// Conn nil and the scraper's deferred Close crashed the whole collector. A
// freshly constructed wrapper has the identical nil-Conn state.
func TestCloseWithNilConnDoesNotPanic(t *testing.T) {
	w := newGoSNMPWrapper()
	require.NotPanics(t, func() {
		require.NoError(t, w.Close())
	})
}

// TestCloseIsIdempotent covers the double close the reset path plus the
// scraper's deferred Close produce on a live connection.
func TestCloseIsIdempotent(t *testing.T) {
	w := newGoSNMPWrapper()
	w.SetTarget("127.0.0.1")
	w.SetPort(30161)
	w.SetTransport("udp")
	w.SetVersion(gosnmp.Version2c)
	w.SetCommunity("public")
	require.NoError(t, w.Connect())
	require.NoError(t, w.Close())
	require.NotPanics(t, func() {
		require.NoError(t, w.Close())
	})
}
