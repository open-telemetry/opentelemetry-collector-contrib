// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

// This file is Linux-only. net.Listen completes the TCP handshake in the kernel
// before Accept, so a portable test server cannot stall the handshake. The stall
// below uses a zero-length accept queue and SOCK_CLOEXEC, which are Linux-specific.
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

func TestRetry_TCPHandshakeDeadlineExceeded(t *testing.T) {
	addr := listenIgnoringHandshake(t)
	dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
	const maxRetries = 1
	endpoint := "http://" + addr
	client, dials := newRetryClient(t, endpoint, time.Second, dialer.DialContext)

	req, err := http.NewRequest(http.MethodPost, endpoint+"/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	var opErr *net.OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, "dial", opErr.Op)
	var dnsErr *net.DNSError
	require.NotErrorAs(t, err, &dnsErr)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout())
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

func listenIgnoringHandshake(t *testing.T) string {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Close(fd) })
	require.NoError(t, syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1))
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}))
	require.NoError(t, syscall.Listen(fd, 0))

	sa, err := syscall.Getsockname(fd)
	require.NoError(t, err)
	addr := fmt.Sprintf("127.0.0.1:%d", sa.(*syscall.SockaddrInet4).Port)

	hold, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = hold.Close() })
	return addr
}
