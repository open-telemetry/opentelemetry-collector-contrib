// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

func TestDetermineNextEventMode(t *testing.T) {
	cases := []struct {
		name          string
		event         func() []byte
		expectedMode  eventMode
		expectedError error
	}{
		{
			"basic",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, "my-tag")
				b = msgp.AppendInt(b, 5000)
				return b
			},
			messageMode,
			nil,
		},
		{
			"str8-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 128))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			messageMode,
			nil,
		},
		{
			"str16-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 1024))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			messageMode,
			nil,
		},
		{
			"str32-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 66000))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			messageMode,
			nil,
		},
		{
			"non-string-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendInt(b, 10)
				b = msgp.AppendInt(b, 5000)
				return b
			},
			unknownMode,
			errors.New("malformed tag field"),
		},
		{
			"float-second-elm",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, "my-tag")
				b = msgp.AppendFloat64(b, 5000.0)
				return b
			},
			unknownMode,
			errors.New("unable to determine next event mode for type float64"),
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			peeker := bufio.NewReaderSize(bytes.NewReader(c.event()), 1024*100)
			mode, err := determineNextEventMode(peeker)
			if c.expectedError != nil {
				require.Equal(t, c.expectedError, err)
			} else {
				require.Equal(t, c.expectedMode, mode)
			}
		})
	}
}

type mockListener struct {
	messagesSent *atomic.Int64
	closed       *atomic.Bool
}

func (m *mockListener) Accept() (net.Conn, error) {
	time.Sleep(10 * time.Millisecond)
	if m.closed.Load() {
		return nil, errors.New("listener closed")
	}
	return &mockConn{
		messagesSent: m.messagesSent,
		closed:       m.closed,
	}, nil
}

func (*mockListener) Close() error {
	return nil
}

func (*mockListener) Addr() net.Addr {
	return &net.TCPAddr{Port: 1234}
}

type mockConn struct {
	messagesSent *atomic.Int64
	closed       *atomic.Bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	msg := createMessage()
	copy(b, msg)
	if m.closed.Load() {
		return 0, errors.New("connection closed")
	}
	m.messagesSent.Add(1)
	return len(b), nil
}

func (*mockConn) Write(_ []byte) (n int, err error) {
	return 0, nil
}

func (*mockConn) Close() error {
	return nil
}

func (*mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{Port: 1234}
}

func (*mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{Port: 1234}
}

func (*mockConn) SetDeadline(_ time.Time) error {
	return nil
}

func (*mockConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (*mockConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func TestServerLifecycle(t *testing.T) {
	eventChannel := make(chan event)
	count := int64(0)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-eventChannel:
				atomic.AddInt64(&count, 1)
			case <-stop:
				return
			}
		}
	}()
	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	s := newServer(eventChannel, zap.NewNop(), telemetryBuilder)
	counter := &atomic.Int64{}
	closed := &atomic.Bool{}
	closed.Store(false)
	ml := &mockListener{
		messagesSent: counter,
		closed:       closed,
	}
	s.Start(ml)
	// wait a second for messages to accumulate
	time.Sleep(1 * time.Second)
	// shutdown
	s.Shutdown()
	// stop sending messages
	closed.Store(true)
	require.Equal(t, atomic.LoadInt64(&count), ml.messagesSent.Load())
}
