// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

// These tests run only on Linux. They bind a TCP socket with SOCK_CLOEXEC and
// do not listen, so a dial is refused and nothing else can take the port.
package elasticsearchexporter

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_UnreachableServer(t *testing.T) {
	endpoint := "http://" + closedLocalAddr(t)

	const maxRetries = 1
	client, dials := newRetryClient(t, endpoint, time.Second, (&net.Dialer{}).DialContext)

	req, err := http.NewRequest(http.MethodPost, endpoint+"/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	var opErr *net.OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, "dial", opErr.Op)
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

// closedLocalAddr binds 127.0.0.1 and does not listen. The socket holds the port
// until cleanup, so a dial is refused and nothing else can bind it.
func closedLocalAddr(t *testing.T) string {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Close(fd) })
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}))
	sa, err := syscall.Getsockname(fd)
	require.NoError(t, err)
	return fmt.Sprintf("127.0.0.1:%d", sa.(*syscall.SockaddrInet4).Port)
}
