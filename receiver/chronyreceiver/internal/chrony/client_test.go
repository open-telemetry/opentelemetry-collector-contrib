// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockConn(tb testing.TB, serverReaderFn, serverWriterFn func(net.Conn) error) net.Conn {
	client, server := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)

	tb.Cleanup(func() {
		wg.Wait()
		assert.NoError(tb, server.Close(), "Must not error when closing server connection")
	})
	assert.NoError(tb, server.SetDeadline(time.Now().Add(time.Second)), "Must not error when assigning deadline")

	go func() {
		defer wg.Done()

		if serverReaderFn == nil {
			serverReaderFn = func(conn net.Conn) error {
				return binary.Read(conn, binary.BigEndian, &requestTrackingContent{})
			}
		}
		assert.NoError(tb, serverReaderFn(server), "Must not error when reading binary data")

		if serverWriterFn == nil {
			serverWriterFn = func(net.Conn) error {
				return nil
			}
		}
		assert.NoError(tb, serverWriterFn(server), "Must not error when processing request")
	}()
	return client
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		addr     string
		toError  bool
	}{
		{scenario: "valid host", addr: "udp://localhost:323", toError: false},
		{scenario: "missing port", addr: "udp://localhost", toError: true},
		{scenario: "missing protocol", addr: "localhost:323", toError: true},
		{scenario: "existing socket to potentially connect to", addr: fmt.Sprint("unix://", t.TempDir()), toError: false},
		{scenario: "invalid path", addr: "unix:///no/socket", toError: true},
		{scenario: "invalid protocol", addr: "http:/localhost:323", toError: true},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			cl, err := New(tc.addr, time.Second)
			if tc.toError {
				assert.Error(t, err, "Must error")
				assert.Nil(t, cl, "Must have a nil client")
			} else {
				assert.NoError(t, err, "Must not error")
				assert.NotNil(t, cl, "Must have a client")
			}
		})
	}
}

func newShortSocketDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return t.TempDir()
	}
	// Intentionally avoid t.TempDir here because unix socket paths must stay as short
	// as possible on some platforms.
	dir, err := os.MkdirTemp("/tmp", "chrony-") //nolint:usetesting
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestWithFileMountPath(t *testing.T) {
	t.Parallel()

	sockDir := newShortSocketDir(t)

	c, err := New("unix://"+sockDir, 10*time.Second, WithFileMountPath(sockDir))
	require.NoError(t, err, "Must not error when creating client")

	cl, ok := c.(*client)
	require.True(t, ok, "Must be a *client")

	assert.Contains(t, cl.localAddr, "otel-chrony-")
	assert.Equal(t, sockDir, filepath.Dir(cl.localAddr), "Must be placed in the specified directory")
}

type localAddrGeneratorSetter interface {
	setLocalAddrGenerator(func(string) (string, error))
}

func TestNewReturnsErrorWhenSocketNameGenerationFails(t *testing.T) {
	t.Parallel()

	sockDir := t.TempDir()
	wantErr := errors.New("entropy unavailable")

	_, err := New("unix://"+sockDir, 10*time.Second,
		WithFileMountPath(sockDir),
		func(c *client) {
			setter, ok := any(c).(localAddrGeneratorSetter)
			require.True(t, ok, "client must allow test override of local socket name generation")
			setter.setLocalAddrGenerator(func(string) (string, error) {
				return "", wantErr
			})
		},
	)
	require.ErrorIs(t, err, wantErr)
}

func TestNewRejectsTooLongFileMountPath(t *testing.T) {
	t.Parallel()

	endpoint := t.TempDir()
	fileMountPath := t.TempDir()
	for len(filepath.Join(fileMountPath, "otel-chrony-deadbeef.sock")) < len(syscall.RawSockaddrUnix{}.Path) {
		fileMountPath = filepath.Join(fileMountPath, "1234567890")
	}
	require.NoError(t, os.MkdirAll(fileMountPath, 0o755))

	_, err := New("unix://"+endpoint, 10*time.Second, WithFileMountPath(fileMountPath))
	require.Error(t, err, "Must reject file_mount_path values that exceed platform socket path limits")
	assert.Contains(t, err.Error(), "file_mount_path")
}

func newTrackingPayload(t *testing.T) []byte {
	return newTrackingPayloadWithRefID(t, 100)
}

func newTrackingPayloadWithRefID(t *testing.T, refID uint32) []byte {
	t.Helper()
	type response struct {
		ReplyHead
		replyTrackingContent
	}
	resp := &response{
		ReplyHead: ReplyHead{
			Version: 6,
			Status:  successfulRequest,
			Reply:   replyTrackingCode,
		},
		replyTrackingContent: replyTrackingContent{
			RefID: refID,
			IPAddr: ipAddr{
				IP:     [16]uint8{127, 0, 0, 1},
				Family: ipAddrInet4,
			},
			Stratum: 10,
		},
	}
	w := &bytes.Buffer{}
	require.NoError(t, binary.Write(w, binary.BigEndian, resp))
	buf := w.Bytes()
	// Pad to 1024 bytes to match client read buffer
	if len(buf) < 1024 {
		buf = append(buf, make([]byte, 1024-len(buf))...)
	}
	return buf
}

type stubAddr string

func (a stubAddr) Network() string { return string(a) }
func (a stubAddr) String() string  { return string(a) }

type scriptedConn struct {
	reads  [][]byte
	closed bool
}

func newScriptedConn(reads ...[]byte) *scriptedConn {
	return &scriptedConn{reads: reads}
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	if len(c.reads) == 0 {
		return 0, io.EOF
	}
	next := c.reads[0]
	c.reads = c.reads[1:]
	return copy(p, next), nil
}

func (c *scriptedConn) Write(p []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	return len(p), nil
}

func (c *scriptedConn) Close() error {
	c.closed = true
	return nil
}

func (*scriptedConn) LocalAddr() net.Addr             { return stubAddr("local") }
func (*scriptedConn) RemoteAddr() net.Addr            { return stubAddr("remote") }
func (*scriptedConn) SetDeadline(time.Time) error     { return nil }
func (*scriptedConn) SetReadDeadline(time.Time) error { return nil }
func (*scriptedConn) SetWriteDeadline(time.Time) error {
	return nil
}

func TestGetTrackingDataRemovesGeneratedSocketOnSuccess(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping UDS test on windows")
	}

	sockDir := newShortSocketDir(t)

	tracking := newTrackingPayload(t)

	c, err := New("unix://"+sockDir, 10*time.Second,
		WithFileMountPath(sockDir),
		func(c *client) {
			c.dialer = func(context.Context, string, string) (net.Conn, error) {
				// Create a mock socket file to simulate dialer binding
				_ = os.WriteFile(c.localAddr, []byte(""), 0o600)
				conn := newMockConn(t,
					nil,
					func(conn net.Conn) error {
						_, writeErr := conn.Write(tracking)
						return writeErr
					},
				)
				return conn, nil
			}
		},
	)
	require.NoError(t, err, "Must not error when creating client")

	_, err = c.GetTrackingData(t.Context())
	require.NoError(t, err, "Must not error when getting tracking data")

	cl := c.(*client)
	expectedAddr := cl.localAddr

	_, err = os.Stat(expectedAddr)
	assert.ErrorIs(t, err, os.ErrNotExist, "Must remove generated socket after a successful scrape")

	require.NoError(t, c.Close(), "Close should remain safe and idempotent after scrape cleanup")
}

func TestWithFileMountPathDoesNotRemoveExistingSockets(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping UDS test on windows")
	}

	sockDir := newShortSocketDir(t)

	// Simulate another receiver already listening on a matching socket path.
	peerPath := filepath.Join(sockDir, "otel-chrony-peer.sock")
	listener, err := net.Listen("unix", peerPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
	})

	// Creating a second client must not delete another receiver's socket.
	_, err = New("unix://"+sockDir, 10*time.Second, WithFileMountPath(sockDir))
	require.NoError(t, err)

	_, err = os.Stat(peerPath)
	assert.NoError(t, err, "Existing receiver socket must not be removed on startup")
}

func TestDialErrorCleansUpLocalSocket(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping UDS test on windows")
	}

	sockDir := newShortSocketDir(t)
	tracking := newTrackingPayload(t)
	attempts := 0

	chronyClient, err := New("unix://"+sockDir, 10*time.Second,
		WithFileMountPath(sockDir),
		func(c *client) {
			c.dialer = func(context.Context, string, string) (net.Conn, error) {
				attempts++
				if attempts == 1 {
					require.NoError(t, os.WriteFile(c.localAddr, []byte(""), 0o600))
					return nil, os.ErrPermission
				}
				return newMockConn(t,
					nil,
					func(conn net.Conn) error {
						_, writeErr := conn.Write(tracking)
						return writeErr
					},
				), nil
			}
		},
	)
	require.NoError(t, err, "Must not error when creating client")

	_, err = chronyClient.GetTrackingData(t.Context())
	assert.ErrorIs(t, err, os.ErrPermission, "Must return the dial error")

	cl := chronyClient.(*client)
	_, err = os.Stat(cl.localAddr)
	require.ErrorIs(t, err, os.ErrNotExist, "Must remove local socket after dial error")

	_, err = chronyClient.GetTrackingData(t.Context())
	require.NoError(t, err, "Must recover on the next dial attempt")
	assert.Equal(t, 2, attempts, "Must retry dialing after cleanup")
}

func TestReadErrorCleansUpGeneratedSocket(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping UDS test on windows")
	}

	sockDir := newShortSocketDir(t)
	readConn := newScriptedConn()

	chronyClient, err := New("unix://"+sockDir, 10*time.Second,
		WithFileMountPath(sockDir),
		func(c *client) {
			c.dialer = func(context.Context, string, string) (net.Conn, error) {
				require.NoError(t, os.WriteFile(c.localAddr, []byte(""), 0o600))
				return readConn, nil
			}
		},
	)
	require.NoError(t, err, "Must not error when creating client")

	_, err = chronyClient.GetTrackingData(t.Context())
	assert.ErrorIs(t, err, io.EOF, "Must return the read error")

	cl := chronyClient.(*client)
	_, err = os.Stat(cl.localAddr)
	require.ErrorIs(t, err, os.ErrNotExist, "Must remove local socket after read error")
	assert.True(t, readConn.closed, "Must close the connection after the read error")
}

func TestGetTrackingDataDialsFreshConnectionPerScrape(t *testing.T) {
	t.Parallel()

	firstConn := newScriptedConn(
		newTrackingPayloadWithRefID(t, 100),
		newTrackingPayloadWithRefID(t, 100),
	)
	secondConn := newScriptedConn(newTrackingPayloadWithRefID(t, 200))

	dialCount := 0
	client, err := New("udp://localhost:323", 10*time.Second, func(c *client) {
		c.dialer = func(context.Context, string, string) (net.Conn, error) {
			dialCount++
			switch dialCount {
			case 1:
				return firstConn, nil
			case 2:
				return secondConn, nil
			default:
				return nil, fmt.Errorf("unexpected dial %d", dialCount)
			}
		}
	})
	require.NoError(t, err, "Must not error when creating client")

	first, err := client.GetTrackingData(t.Context())
	require.NoError(t, err, "Must not error when getting first tracking sample")
	assert.Equal(t, uint32(100), first.RefID, "Must read the first connection's reply")

	second, err := client.GetTrackingData(t.Context())
	require.NoError(t, err, "Must not error when getting second tracking sample")
	assert.Equal(t, uint32(200), second.RefID, "Must dial a fresh connection instead of reusing a stale queued reply")
	assert.Equal(t, 2, dialCount, "Must dial once per scrape")
	assert.True(t, firstConn.closed, "Must close the first scrape connection before the next scrape")
	assert.True(t, secondConn.closed, "Must close the second scrape connection after the scrape completes")
}

func TestGettingTrackingData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario       string
		serverReaderFn func(conn net.Conn) error
		serverWriterFn func(conn net.Conn) error
		dialTime       time.Duration
		timeout        time.Duration
		data           *Tracking
		err            error
	}{
		{
			scenario: "Successful read binary from socket",
			timeout:  10 * time.Second,
			serverWriterFn: func(conn net.Conn) error {
				type response struct {
					ReplyHead
					replyTrackingContent
				}
				resp := &response{
					ReplyHead: ReplyHead{
						Version: 6,
						Status:  successfulRequest,
						Reply:   replyTrackingCode,
					},
					replyTrackingContent: replyTrackingContent{
						RefID: 100,
						IPAddr: ipAddr{
							IP:     [16]uint8{127, 0, 0, 1},
							Family: ipAddrInet4,
						},
						Stratum:    10,
						LeapStatus: 0,
						RefTime: timeSpec{
							100, 10, 0,
						},
						CurrentCorrection:  binaryFloat(1300),
						LastOffset:         binaryFloat(10000),
						RMSOffset:          binaryFloat(12000),
						FreqPPM:            binaryFloat(3300),
						ResidFreqPPM:       binaryFloat(123456),
						SkewPPM:            binaryFloat(9943),
						RootDelay:          binaryFloat(-1220),
						RootDispersion:     binaryFloat(-1100000),
						LastUpdateInterval: binaryFloat(120),
					},
				}

				return binary.Write(conn, binary.BigEndian, resp)
			},
			data: &Tracking{
				RefID:              100,
				IPAddr:             net.IP([]byte{127, 0, 0, 1}),
				Stratum:            10,
				LeapStatus:         0,
				RefTime:            (&timeSpec{100, 10, 0}).Time(),
				CurrentCorrection:  binaryFloat(1300).Float(),
				LastOffset:         binaryFloat(10000).Float(),
				RMSOffset:          binaryFloat(12000).Float(),
				FreqPPM:            binaryFloat(3300).Float(),
				ResidFreqPPM:       binaryFloat(123456).Float(),
				SkewPPM:            binaryFloat(9943).Float(),
				RootDelay:          binaryFloat(-1220).Float(),
				RootDispersion:     binaryFloat(-1100000).Float(),
				LastUpdateInterval: binaryFloat(120).Float(),
			},
			err: nil,
		},
		{
			scenario: "Timeout waiting for dial",
			timeout:  10 * time.Millisecond,
			dialTime: 100 * time.Millisecond,
			serverReaderFn: func(net.Conn) error {
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "Timeout waiting for response",
			timeout:  10 * time.Millisecond,
			serverReaderFn: func(net.Conn) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "Timeout waiting for response because of slow dial",
			timeout:  100 * time.Millisecond,
			dialTime: 90 * time.Millisecond,
			serverWriterFn: func(net.Conn) error {
				time.Sleep(20 * time.Millisecond)
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "invalid status returned",
			timeout:  5 * time.Second,
			serverWriterFn: func(conn net.Conn) error {
				resp := &ReplyHead{
					Version: 6,
					Status:  1,
					Reply:   replyTrackingCode,
				}
				return binary.Write(conn, binary.BigEndian, resp)
			},
			err: errBadRequest,
		},
		{
			scenario: "invalid status command",
			timeout:  5 * time.Second,
			serverWriterFn: func(conn net.Conn) error {
				resp := &ReplyHead{
					Version: 6,
					Status:  successfulRequest,
					Reply:   0,
				}
				return binary.Write(conn, binary.BigEndian, resp)
			},
			err: errBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			client, err := New("unix://"+t.TempDir(), tc.timeout, func(c *client) {
				c.dialer = func(context.Context, string, string) (net.Conn, error) {
					if tc.dialTime > tc.timeout {
						return nil, os.ErrDeadlineExceeded
					}

					return newMockConn(t, tc.serverReaderFn, tc.serverWriterFn), nil
				}
			})
			require.NoError(t, err, "Must not error when creating client")

			data, err := client.GetTrackingData(t.Context())
			assert.Equal(t, tc.data, data, "Must match the expected data")
			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
		})
	}
}
