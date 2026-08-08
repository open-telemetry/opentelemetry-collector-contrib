// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type closeTrackingConn struct {
	net.Conn
	closed int32
}

func (c *closeTrackingConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	return c.Conn.Close()
}

// Test_handleTCPConn_ClosesConnectionOnClientDisconnect checks that
// handleTCPConn closes the accepted connection once the client disconnects,
// even for ordinary, well-behaved traffic (one valid line, then a clean
// close). Without this, every TCP connection's server-side file descriptor
// leaks for as long as the process runs.
func Test_handleTCPConn_ClosesConnectionOnClientDisconnect(t *testing.T) {
	server, client := net.Pipe()
	tracked := &closeTrackingConn{Conn: server}

	transferChan := make(chan Metric, 10)
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range transferChan {
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		handleTCPConn(tracked, NewMockReporter(0), transferChan)
	}()

	_, err := client.Write([]byte("ordinary.client.metric:1|c\n"))
	require.NoError(t, err)
	require.NoError(t, client.Close())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleTCPConn did not return after the client disconnected")
	}
	close(transferChan)
	<-drainDone

	require.Equal(t, int32(1), atomic.LoadInt32(&tracked.closed),
		"handleTCPConn must close the server-side connection once the client disconnects")
}

// Test_handleTCPConn_BoundsUnterminatedLineSize checks that a client
// streaming data without ever sending '\n' cannot make handleTCPConn buffer
// an unbounded amount of memory in its `remainder` accumulator.
func Test_handleTCPConn_BoundsUnterminatedLineSize(t *testing.T) {
	server, client := net.Pipe()
	transferChan := make(chan Metric, 10)
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range transferChan {
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleTCPConn(server, NewMockReporter(0), transferChan)
	}()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const totalSize = 32 * 1024 * 1024 // 32MB, single logical line, no '\n'
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 'A'
	}

	sendDone := make(chan struct{})
	go func() {
		defer close(sendDone)
		sent := 0
		for sent < totalSize {
			n, err := client.Write(chunk)
			if err != nil {
				return
			}
			sent += n
		}
	}()

	// Snapshot heap while the connection is still open, i.e. while any
	// unbounded `remainder` buffer would still be live.
	<-sendDone
	runtime.GC()
	var peak runtime.MemStats
	runtime.ReadMemStats(&peak)

	client.Close()
	wg.Wait()
	close(transferChan)
	<-drainDone

	peakDeltaMB := int64(peak.HeapAlloc-before.HeapAlloc) / (1024 * 1024)
	t.Logf("sent %dMB with zero newlines over one connection; heap grew by %dMB while the connection was open", totalSize/(1024*1024), peakDeltaMB)

	const boundMB = 4
	require.LessOrEqualf(t, peakDeltaMB, int64(boundMB),
		"a single unterminated-line connection must not buffer more than ~maxLineSize bytes, got %dMB of heap growth", peakDeltaMB)
}
