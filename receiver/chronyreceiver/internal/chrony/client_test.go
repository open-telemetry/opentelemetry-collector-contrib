// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockConn(tb testing.TB, handler func(net.Conn) error) net.Conn {
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

		assert.NoError(tb, binary.Read(server, binary.BigEndian, &requestTrackingContent{}), "Must not error when reading binary data")
		assert.NoError(tb, handler(server), "Must not error when processing request")
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
		tc := tc
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

func TestGettingTrackingData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		handler  func(conn net.Conn) error
		dialTime time.Duration
		timeout  time.Duration
		data     *Tracking
		err      error
	}{
		{
			scenario: "Successful read binary from socket",
			timeout:  10 * time.Second,
			handler: func(conn net.Conn) error {
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
			handler: func(conn net.Conn) error {
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "Timeout waiting for response",
			timeout:  10 * time.Millisecond,
			handler: func(conn net.Conn) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "Timeout waiting for response because of slow dial",
			timeout:  100 * time.Millisecond,
			dialTime: 90 * time.Millisecond,
			handler: func(conn net.Conn) error {
				time.Sleep(20 * time.Millisecond)
				return nil
			},
			err: os.ErrDeadlineExceeded,
		},
		{
			scenario: "invalid status returned",
			timeout:  5 * time.Second,
			handler: func(conn net.Conn) error {
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
			handler: func(conn net.Conn) error {
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
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			client, err := New(fmt.Sprintf("unix://%s", t.TempDir()), tc.timeout, func(c *client) {
				c.dialer = func(ctx context.Context, _, _ string) (net.Conn, error) {
					if tc.dialTime > tc.timeout {
						return nil, os.ErrDeadlineExceeded
					}

					return newMockConn(t, tc.handler), nil
				}
			})
			require.NoError(t, err, "Must not error when creating client")

			data, err := client.GetTrackingData(context.Background())
			assert.EqualValues(t, tc.data, data, "Must match the expected data")
			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
		})
	}
}
