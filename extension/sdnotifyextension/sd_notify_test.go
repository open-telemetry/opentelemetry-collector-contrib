// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap/zaptest"
)

// noopHost is a minimal component.Host for unit tests.
type noopHost struct{}

func (noopHost) GetExtensions() map[component.ID]component.Component { return nil }

// startFakeNotifySocket opens a unix socket, points $NOTIFY_SOCKET at it, and
// returns a channel that receives every payload systemd would have seen.
func startFakeNotifySocket(t *testing.T) <-chan string {
	t.Helper()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "notify.sock")

	conn, err := net.ListenPacket("unixgram", sockPath)
	require.NoError(t, err, "listen on fake NOTIFY_SOCKET")
	t.Cleanup(func() { _ = conn.Close() })

	t.Setenv("NOTIFY_SOCKET", sockPath)

	msgs := make(chan string, 8)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			msgs <- string(buf[:n])
		}
	}()
	return msgs
}

func TestStart_NoNotifySocket_IsNoop(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")
	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))
	require.NoError(t, s.Shutdown(context.Background()))
}

func TestShutdown_BeforeStart_IsNoop(t *testing.T) {
	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Shutdown(context.Background()))
}

func TestShutdown_IsIdempotent(t *testing.T) {
	_ = startFakeNotifySocket(t)

	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))

	require.NoError(t, s.Shutdown(context.Background()))
	require.NoError(t, s.Shutdown(context.Background()))
}

func TestReady_SendsREADY(t *testing.T) {
	msgs := startFakeNotifySocket(t)

	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	require.NoError(t, s.Ready())
	select {
	case got := <-msgs:
		require.Equal(t, "READY=1", got)
	case <-time.After(2 * time.Second):
		t.Fatal("no datagram received on fake NOTIFY_SOCKET")
	}
}

func TestSIGTERM_SendsSTOPPING(t *testing.T) {
	msgs := startFakeNotifySocket(t)

	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	select {
	case got := <-msgs:
		require.Equal(t, "STOPPING=1", got)
	case <-time.After(2 * time.Second):
		t.Fatal("no STOPPING=1 datagram received after SIGTERM")
	}
}

func TestSIGHUP_SendsRELOADING(t *testing.T) {
	msgs := startFakeNotifySocket(t)

	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))
	select {
	case got := <-msgs:
		require.Contains(t, got, "RELOADING=1")
		require.Contains(t, got, "MONOTONIC_USEC=")
	case <-time.After(2 * time.Second):
		t.Fatal("no event received at NOTIFY_SOCKET after SIGHUP")
	}
}

func TestWatchdog_SendsWATCHDOG(t *testing.T) {
	msgs := startFakeNotifySocket(t)

	t.Setenv("WATCHDOG_USEC", "100000") // 100ms
	t.Setenv("WATCHDOG_PID", strconv.Itoa(os.Getpid()))

	s := newSDNotify(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, s.Start(context.Background(), noopHost{}))
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 325*time.Millisecond)
	defer cancel()

	// We expect to receive 6 notifications because:
	//   - WATCHDOG_USEC is set to 100ms, so a notification is sent every 50ms.
	//   - Over a 300ms interval, this results in 300 / 50 = 6 notifications.
	//
	// We wait 325ms instead of exactly 300ms to avoid timing-related flakiness.
	// Waiting exactly 300ms could cause the test to occasionally miss the last
	// notification due to scheduling or timer jitter.
	//
	// Note: We cannot use testing/synctest here because it does not support
	// signal.Notify. See: https://github.com/golang/go/issues/78494
	count := 0
	for {
		select {
		case got := <-msgs:
			require.Equal(t, "WATCHDOG=1", got)
			count++
		case <-ctx.Done():
			require.Equal(t, 6, count,
				"expected 6 WATCHDOG=1 notifications within 300ms, got %d", count)
			return
		}
	}
}
