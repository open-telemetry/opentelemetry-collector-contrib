// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package transport

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewUDSServer_ListenPacketFailure(t *testing.T) {
	invalidPath := "/invalid_path/test_socket"

	server, err := NewUDSServer("unixgram", invalidPath, 0o622)

	assert.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "starting to listen")
}

func Test_UDSServer_Close(t *testing.T) {
	socketPath := "/tmp/test_socket_close"
	defer os.Remove(socketPath)

	server, err := NewUDSServer("unixgram", socketPath, 0o622)
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

	server, err := NewUDSServer("unixgram", socketPath, expectedPermissions)
	require.NoError(t, err)
	require.NotNil(t, server)

	fileInfo, err := os.Stat(socketPath)
	require.NoError(t, err)

	actualPermissions := fileInfo.Mode().Perm()
	assert.Equal(t, expectedPermissions, actualPermissions, "Expected file permissions to be set correctly")

	server.Close()
}
