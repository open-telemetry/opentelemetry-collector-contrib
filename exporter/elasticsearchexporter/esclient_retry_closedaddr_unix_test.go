// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package elasticsearchexporter

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// closedLocalAddr binds 127.0.0.1 and does not listen. The socket holds the port
// until cleanup, so a dial is refused and nothing else can bind it.
func closedLocalAddr(t *testing.T) string {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	syscall.CloseOnExec(fd)
	t.Cleanup(func() { _ = syscall.Close(fd) })
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}))
	sa, err := syscall.Getsockname(fd)
	require.NoError(t, err)
	return fmt.Sprintf("127.0.0.1:%d", sa.(*syscall.SockaddrInet4).Port)
}
