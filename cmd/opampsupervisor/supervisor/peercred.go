// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"

import (
	"fmt"
	"net"
	"os"

	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"go.uber.org/zap"
)

// verifyAgentConn authenticates the peer of a Unix domain socket connection
// against the collector the supervisor spawned. It verifies that the peer's UID
// matches the supervisor's own UID (the collector is spawned as the same user)
// and, on platforms that expose it (Linux via SO_PEERCRED), that the peer's PID
// matches the expected collector PID.
//
// getExpectedPID returns the PID the connecting collector is expected to have,
// or 0 when no collector is running; connections are rejected in that case (the
// PID is recorded before the spawned collector can possibly dial, so nothing
// legitimate ever connects while it is 0). The credentials returned by the
// kernel reflect the actual connecting process and cannot be spoofed by the
// peer.
func (s *Supervisor) verifyAgentConn(conn serverTypes.Connection, getExpectedPID func() int) bool {
	netConn := conn.Connection()
	unixConn, ok := netConn.(*net.UnixConn)
	if !ok {
		s.telemetrySettings.Logger.Error(
			"rejecting OpAMP connection: peer is not a unix socket",
			zap.String("conn_type", fmt.Sprintf("%T", netConn)),
		)
		return false
	}

	wantPID := 0
	if getExpectedPID != nil {
		wantPID = getExpectedPID()
	}
	if wantPID == 0 {
		s.telemetrySettings.Logger.Warn("rejecting OpAMP connection: no collector is currently running")
		return false
	}

	if err := verifyPeerCredentials(unixConn, os.Getuid(), wantPID); err != nil {
		s.telemetrySettings.Logger.Warn("rejecting OpAMP connection from unauthenticated peer", zap.Error(err))
		return false
	}
	return true
}

// authenticateAgentPeer returns an OnConnected hook that disconnects
// unauthenticated peers. This gates the WebSocket transport, whose read loop
// starts after OnConnected returns. It does NOT gate the plain-HTTP OpAMP
// transport, where Disconnect is a no-op and the message is processed anyway;
// requirePeerAuth covers that path.
func (s *Supervisor) authenticateAgentPeer(getExpectedPID func() int) func(serverTypes.Connection) {
	return func(conn serverTypes.Connection) {
		if !s.verifyAgentConn(conn, getExpectedPID) {
			_ = conn.Disconnect()
		}
	}
}

// requirePeerAuth wraps an OpAMP message handler so that messages from
// unauthenticated peers are dropped. This is required in addition to the
// OnConnected hook because opamp-go serves the plain-HTTP transport on the same
// listener and processes the request's message regardless of OnConnected
// (Disconnect is a no-op for HTTP connections).
func (s *Supervisor) requirePeerAuth(
	getExpectedPID func() int,
	next func(serverTypes.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent,
) func(serverTypes.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent {
	return func(conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
		if !s.verifyAgentConn(conn, getExpectedPID) {
			_ = conn.Disconnect()
			return &protobufs.ServerToAgent{}
		}
		return next(conn, message)
	}
}
