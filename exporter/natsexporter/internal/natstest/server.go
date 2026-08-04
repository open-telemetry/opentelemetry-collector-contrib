// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package natstest provides helpers for spinning up an in-process NATS server
// in tests, so the exporter can be exercised against a real broker without an
// external dependency.
package natstest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/natstest"

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

// NewServer starts an in-process core NATS server on a random port and returns
// its client URL. The server is shut down automatically when the test ends.
func NewServer(tb testing.TB) string {
	tb.Helper()
	return newServer(tb, &server.Options{
		Host:   "127.0.0.1",
		Port:   -1, // pick a random available port
		NoLog:  true,
		NoSigs: true,
	})
}

// NewJetStreamServer starts an in-process NATS server with JetStream enabled.
func NewJetStreamServer(tb testing.TB) string {
	tb.Helper()
	return newServer(tb, &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  tb.TempDir(),
	})
}

func newServer(tb testing.TB, opts *server.Options) string {
	tb.Helper()
	srv, err := server.NewServer(opts)
	require.NoError(tb, err)
	go srv.Start()
	tb.Cleanup(srv.Shutdown)
	require.Truef(tb, srv.ReadyForConnections(5*time.Second), "nats server not ready for connections")
	return srv.ClientURL()
}
