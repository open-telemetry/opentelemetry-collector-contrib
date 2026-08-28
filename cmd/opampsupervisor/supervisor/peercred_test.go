// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package supervisor

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// unixConnPair returns the server side of a connected Unix domain socket pair.
// The peer (client) is the test process itself, so its kernel credentials match
// the current process.
func unixConnPair(t *testing.T) *net.UnixConn {
	t.Helper()

	// Use a short base dir: macOS limits the socket path to ~104 bytes.
	dir, err := os.MkdirTemp("/tmp", "peercred")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ln, err := net.Listen("unix", filepath.Join(dir, "s.sock"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	dialed := make(chan net.Conn, 1)
	go func() {
		c, derr := net.Dial("unix", ln.Addr().String())
		require.NoError(t, derr)
		dialed <- c
	}()

	srvConn, err := ln.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = srvConn.Close() })

	clientConn := <-dialed
	t.Cleanup(func() { _ = clientConn.Close() })

	uc, ok := srvConn.(*net.UnixConn)
	require.True(t, ok)
	return uc
}

func TestRemoveStaleUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "stalesock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	t.Run("missing path is a no-op", func(t *testing.T) {
		require.NoError(t, removeStaleUnixSocket(filepath.Join(dir, "does-not-exist.sock")))
	})

	t.Run("removes a stale socket", func(t *testing.T) {
		socketPath := filepath.Join(dir, "stale.sock")
		ln, lerr := net.Listen("unix", socketPath)
		require.NoError(t, lerr)
		require.NoError(t, ln.Close()) // Close may leave the file depending on platform
		// Recreate to guarantee a socket file is present.
		ln2, lerr := net.Listen("unix", socketPath)
		require.NoError(t, lerr)
		t.Cleanup(func() { _ = ln2.Close() })

		require.NoError(t, removeStaleUnixSocket(socketPath))
		_, statErr := os.Stat(socketPath)
		require.ErrorIs(t, statErr, os.ErrNotExist)
	})

	t.Run("refuses to remove a non-socket file", func(t *testing.T) {
		regular := filepath.Join(dir, "regular.txt")
		require.NoError(t, os.WriteFile(regular, []byte("data"), 0o600))

		require.Error(t, removeStaleUnixSocket(regular))
		require.FileExists(t, regular, "a non-socket file must not be removed")
	})

	t.Run("refuses to remove a symlink", func(t *testing.T) {
		target := filepath.Join(dir, "target.txt")
		require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
		link := filepath.Join(dir, "link.sock")
		require.NoError(t, os.Symlink(target, link))

		require.Error(t, removeStaleUnixSocket(link))
		require.FileExists(t, target, "the symlink target must not be removed")
	})
}

func TestVerifyPeerCredentials(t *testing.T) {
	uc := unixConnPair(t)

	// The peer is this test process, so matching the current UID succeeds.
	require.NoError(t, verifyPeerCredentials(uc, os.Getuid(), 0))

	// A mismatched UID is rejected.
	require.Error(t, verifyPeerCredentials(uc, os.Getuid()+1, 0))

	if runtime.GOOS == "linux" {
		// On Linux SO_PEERCRED also exposes the peer PID.
		require.NoError(t, verifyPeerCredentials(uc, os.Getuid(), os.Getpid()))
		require.Error(t, verifyPeerCredentials(uc, os.Getuid(), os.Getpid()+1))
	}
}

// recordingConn is a serverTypes.Connection that exposes a fixed net.Conn and
// records whether Disconnect was called.
type recordingConn struct {
	conn         net.Conn
	disconnected atomic.Bool
}

func (c *recordingConn) Connection() net.Conn { return c.conn }

func (*recordingConn) Send(context.Context, *protobufs.ServerToAgent) error { return nil }

func (c *recordingConn) Disconnect() error {
	c.disconnected.Store(true)
	return nil
}

func newPeerAuthSupervisor() *Supervisor {
	return &Supervisor{
		telemetrySettings: telemetrySettings{
			TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
		},
	}
}

func TestCreateOpAMPServerListenerSocketMode(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "sockmode")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "opamp.sock")

	s := newPeerAuthSupervisor()
	s.config = config.Supervisor{Agent: config.Agent{
		OpAMPServerUnixSocket:     socketPath,
		OpAMPServerUnixSocketMode: "0660",
	}}

	ln, err := s.createOpAMPServerListener()
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	info, err := os.Stat(socketPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o660), info.Mode().Perm(), "socket file must carry the configured mode")
}

func TestAuthenticateAgentPeer_AcceptsSpawnedCollector(t *testing.T) {
	s := newPeerAuthSupervisor()
	conn := &recordingConn{conn: unixConnPair(t)}

	// The peer is this process; supplying its PID satisfies the (Linux) PID check
	// and the UID always matches.
	s.authenticateAgentPeer(func() int { return os.Getpid() })(conn)

	require.False(t, conn.disconnected.Load(), "a matching peer must not be disconnected")
}

func TestAuthenticateAgentPeer_RejectsNonUnixPeer(t *testing.T) {
	s := newPeerAuthSupervisor()
	c1, c2 := net.Pipe()
	t.Cleanup(func() { _ = c1.Close(); _ = c2.Close() })

	conn := &recordingConn{conn: c1} // net.Pipe conns are not *net.UnixConn
	s.authenticateAgentPeer(func() int { return 0 })(conn)

	require.True(t, conn.disconnected.Load(), "a non-unix peer must be disconnected")
}

func TestAuthenticateAgentPeer_RejectsWhenNoCollectorRunning(t *testing.T) {
	s := newPeerAuthSupervisor()
	conn := &recordingConn{conn: unixConnPair(t)}

	// Expected PID 0 means no collector is running; nothing legitimate connects
	// in that state, so the connection must be rejected.
	s.authenticateAgentPeer(func() int { return 0 })(conn)

	require.True(t, conn.disconnected.Load(), "connections must be rejected while no collector is running")
}

func TestRequirePeerAuth(t *testing.T) {
	s := newPeerAuthSupervisor()

	nextCalled := false
	next := func(serverTypes.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent {
		nextCalled = true
		return nil
	}

	// Authenticated peer: the wrapped handler runs.
	conn := &recordingConn{conn: unixConnPair(t)}
	s.requirePeerAuth(func() int { return os.Getpid() }, next)(conn, &protobufs.AgentToServer{})
	require.True(t, nextCalled, "messages from an authenticated peer must reach the handler")
	require.False(t, conn.disconnected.Load())

	// Unauthenticated peer (no collector running): dropped before the handler.
	// This is what gates the plain-HTTP transport, where Disconnect is a no-op.
	nextCalled = false
	conn = &recordingConn{conn: unixConnPair(t)}
	resp := s.requirePeerAuth(func() int { return 0 }, next)(conn, &protobufs.AgentToServer{})
	require.False(t, nextCalled, "messages from an unauthenticated peer must not reach the handler")
	require.True(t, conn.disconnected.Load())
	require.NotNil(t, resp, "a rejected message still gets an empty response")
}

func TestAuthenticateAgentPeer_RejectsWrongPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("peer PID is only available via SO_PEERCRED on Linux")
	}
	s := newPeerAuthSupervisor()
	conn := &recordingConn{conn: unixConnPair(t)}

	// A PID that cannot be the peer's must be rejected.
	s.authenticateAgentPeer(func() int { return os.Getpid() + 1 })(conn)

	require.True(t, conn.disconnected.Load(), "a peer with the wrong PID must be disconnected")
}
