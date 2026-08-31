// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package supervisor

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pipeCounter atomic.Int64

// pipeConnPair returns the server side of a connected named pipe pair. The
// peer (client) is the test process itself, so its kernel-reported client PID
// is the current process's PID.
func pipeConnPair(t *testing.T) net.Conn {
	t.Helper()

	pipePath := fmt.Sprintf(`\\.\pipe\opamp-peercred-%d-%d`, os.Getpid(), pipeCounter.Add(1))
	ln, err := winio.ListenPipe(pipePath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	dialed := make(chan net.Conn, 1)
	go func() {
		c, derr := winio.DialPipe(pipePath, nil)
		assert.NoError(t, derr)
		dialed <- c
	}()

	srvConn, err := ln.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = srvConn.Close() })

	clientConn := <-dialed
	require.NotNil(t, clientConn)
	t.Cleanup(func() { _ = clientConn.Close() })

	return srvConn
}

func TestVerifyPeerCredentialsNamedPipe(t *testing.T) {
	conn := pipeConnPair(t)

	// The peer is this test process, so matching the current PID succeeds.
	require.NoError(t, verifyPeerCredentials(conn, 0, os.Getpid()))

	// A mismatched PID is rejected.
	require.Error(t, verifyPeerCredentials(conn, 0, os.Getpid()+1))
}

func TestVerifyPeerCredentialsNamedPipe_RejectsNonPipeConn(t *testing.T) {
	c1, c2 := net.Pipe()
	t.Cleanup(func() { _ = c1.Close(); _ = c2.Close() })

	// net.Pipe conns expose no named pipe handle and must be rejected.
	require.Error(t, verifyPeerCredentials(c1, 0, os.Getpid()))
}
