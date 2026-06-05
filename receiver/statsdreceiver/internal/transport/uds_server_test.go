// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package transport

import (
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewUDSServer_ListenPacketFailure(t *testing.T) {
	invalidPath := "/invalid_path/test_socket"

	server, err := NewUDSServer("unixgram", invalidPath, 0o622, 0)

	assert.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "starting to listen")
}

func Test_UDSServer_Close(t *testing.T) {
	socketPath := "/tmp/test_socket_close"
	defer os.Remove(socketPath)

	server, err := NewUDSServer("unixgram", socketPath, 0o622, 0)
	require.NoError(t, err)
	require.NotNil(t, server)

	_, err = os.Stat(socketPath)
	require.NoError(t, err)

	err = server.Close()
	assert.NoError(t, err)

	_, err = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(err))
}

func Test_NewUDSServer_AppliesChmod(t *testing.T) {
	socketPath := "/tmp/test_socket_chmod"
	defer os.Remove(socketPath) // Cleanup after test

	expectedPermissions := os.FileMode(0o622)

	server, err := NewUDSServer("unixgram", socketPath, expectedPermissions, 0)
	require.NoError(t, err)
	require.NotNil(t, server)

	fileInfo, err := os.Stat(socketPath)
	require.NoError(t, err)

	actualPermissions := fileInfo.Mode().Perm()
	assert.Equal(t, expectedPermissions, actualPermissions, "Expected file permissions to be set correctly")

	server.Close()
}

func Test_NewUDSServer_SocketBufferSize(t *testing.T) {
	socketPath := "/tmp/test_socket_bufsize"
	defer os.Remove(socketPath)

	bufferSize := 2 * 1024 * 1024 // 2MB

	server, err := NewUDSServer("unixgram", socketPath, 0o622, bufferSize)
	require.NoError(t, err)
	require.NotNil(t, server)
	defer server.Close()

	// Verify the buffer was set by checking the underlying connection.
	udsServer := server.(*udsServer)
	if uc, ok := udsServer.packetConn.(*net.UnixConn); ok {
		raw, err := uc.SyscallConn()
		require.NoError(t, err)
		var actual int
		err = raw.Control(func(fd uintptr) {
			actual, _ = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
		})
		require.NoError(t, err)
		// The kernel may round up, so just check it's at least what we asked for.
		assert.GreaterOrEqual(t, actual, bufferSize,
			"SO_RCVBUF should be at least the requested size")
	}
}

func Test_NewUDSServer_DefaultSocketBufferSize(t *testing.T) {
	socketPath := "/tmp/test_socket_bufsize_default"
	defer os.Remove(socketPath)

	// socketBufferSize=0 means don't change the OS default.
	server, err := NewUDSServer("unixgram", socketPath, 0o622, 0)
	require.NoError(t, err)
	require.NotNil(t, server)
	server.Close()
}
