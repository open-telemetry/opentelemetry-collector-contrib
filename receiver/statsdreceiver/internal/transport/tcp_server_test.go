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
	closed atomic.Int32
}

func (c *closeTrackingConn) Close() error {
	c.closed.Store(1)
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
		for m := range transferChan {
			_ = m // this test only cares about connection lifecycle, not parsed metrics
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

	require.Equal(t, int32(1), tracked.closed.Load(),
		"handleTCPConn must close the server-side connection once the client disconnects")
}

// Test_handleTCPConn_NoStaleRemainderAtReadBoundary is a regression test for
// a bug where remainder was only updated on io.EOF when ReadBytes returned a
// non-empty leftover. When a partial line spans two reads and the second
// read's data ends exactly at the '\n' that completes it, ReadBytes's final
// call in that read returns empty bytes with io.EOF, so remainder would keep
// its old value from the first read - already-consumed data - and wrongly
// get re-prepended to the next read, corrupting and duplicating the
// following line.
func Test_handleTCPConn_NoStaleRemainderAtReadBoundary(t *testing.T) {
	server, client := net.Pipe()
	transferChan := make(chan Metric, 10)

	done := make(chan struct{})
	go func() {
		defer close(done)
		handleTCPConn(server, NewMockReporter(0), transferChan)
	}()

	// Three separate writes, chosen so each is drained by exactly one Read
	// inside handleTCPConn's outer loop: net.Pipe's Write blocks until fully
	// consumed, and each write here is well under the 4096-byte read
	// buffer, so a write can't be split or coalesced with another across
	// reads. This reproduces the exact sequence from the bug: a partial
	// line with no newline, then a read that completes it exactly at the
	// newline, then an independent third line.
	for _, w := range []string{"abc", "def\n", "ghi\n"} {
		_, err := client.Write([]byte(w))
		require.NoError(t, err)
	}
	require.NoError(t, client.Close())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleTCPConn did not return after the client disconnected")
	}
	close(transferChan)

	var got []string
	for m := range transferChan {
		got = append(got, m.Raw)
	}

	// Before the fix, the stale remainder from the first write ("abc")
	// would survive the second write's EOF-with-empty-bytes and get
	// wrongly re-prepended to the third write, producing
	// []string{"abcdef", "abcghi"} instead.
	require.Equal(t, []string{"abcdef", "ghi"}, got,
		"remainder must not survive an EOF where ReadBytes returned no leftover bytes; a stale value corrupts/duplicates the next line")
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
		for m := range transferChan {
			_ = m // this test only cares about the size bound, not parsed metrics
		}
	}()

	var wg sync.WaitGroup
	wg.Go(func() {
		handleTCPConn(server, NewMockReporter(0), transferChan)
	})

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	const totalSize = 32 * 1024 * 1024 // 32MB, single logical line, no '\n'
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 'A'
	}

	var sentBytes atomic.Int64
	sendDone := make(chan struct{})
	go func() {
		defer close(sendDone)
		for sentBytes.Load() < int64(totalSize) {
			n, err := client.Write(chunk)
			// Record n even on error: once handleTCPConn closes the connection
			// (expected here, since we deliberately exceed maxLineSize),
			// net.Pipe's Write can return a non-zero partial count alongside
			// the error, and that partial count is what actually reached the
			// server - it's the number we want for the assertions below.
			sentBytes.Add(int64(n))
			if err != nil {
				return
			}
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

	actualSent := sentBytes.Load()
	peakDeltaMB := int64(peak.HeapAlloc-before.HeapAlloc) / (1024 * 1024)
	t.Logf("client wrote %d bytes with zero newlines before the connection closed (of a possible %dMB); heap grew by %dMB while the connection was open",
		actualSent, totalSize/(1024*1024), peakDeltaMB)

	// The server is expected to close the connection once the unterminated
	// line exceeds maxLineSize, well short of the full totalSize. Assert we
	// actually crossed that threshold so this test can't silently pass by
	// having the client finish writing all 32MB before the bound ever kicks
	// in - which would exercise nothing about the new bounded-line behavior.
	require.Greaterf(t, actualSent, int64(maxLineSize),
		"test must write more than maxLineSize (%d) bytes to actually exercise the bounded-line closure path, only wrote %d", maxLineSize, actualSent)

	const boundMB = 4
	require.LessOrEqualf(t, peakDeltaMB, int64(boundMB),
		"a single unterminated-line connection must not buffer more than ~maxLineSize bytes, got %dMB of heap growth", peakDeltaMB)
}
