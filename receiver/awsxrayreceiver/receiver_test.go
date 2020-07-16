// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsxrayreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestConsumerCantBeNil(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	defer sock.Close()
	address := sock.LocalAddr().String()

	_, err = newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  address,
				Transport: transport,
			},
		},
		nil,
		zap.NewNop(),
	)
	assert.Error(t, err, "should have failed to create a new receiver")
	assert.True(t, errors.Is(err, componenterror.ErrNilNextConsumer), "consumer is nil should be detected")
}

func TestNonUDPTransport(t *testing.T) {
	_, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  "dontCare",
				Transport: "tcp",
			},
		},
		new(exportertest.SinkTraceExporter),
		zap.NewNop(),
	)
	assert.EqualError(t, err,
		"X-Ray receiver only supports ingesting spans through UDP, provided: tcp")
}

func TestUDPPortUnavailable(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	defer sock.Close()
	address := sock.LocalAddr().String()

	sink := new(exportertest.SinkTraceExporter)
	_, err = newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  address,
				Transport: transport,
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.Error(t, err, "should have failed to create a new receiver")
	assert.Contains(t, err.Error(), "address already in use", "error message should complain about address in-use")
}

func TestShutdownStopsPollers(t *testing.T) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	sink := new(exportertest.SinkTraceExporter)
	rcvr, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: transport,
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.NoError(t, err, "receiver should be created")

	// start pollers
	err = rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "should be able to start receiver")

	pollerStops := make(chan bool)
	go func() {
		err = rcvr.Shutdown(context.Background())
		assert.NoError(t, err, "should be able to shutdown the receiver")
		close(pollerStops)
	}()

	testutil.WaitFor(t, func() bool {
		select {
		case _, open := <-pollerStops:
			return !open
		default:
			return false
		}
	}, "poller is not stopped")
}

func TestCantStartAnInstanceTwice(t *testing.T) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	sink := new(exportertest.SinkTraceExporter)
	rcvr, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: transport,
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.NoError(t, err, "receiver should be created")

	// start pollers
	err = rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "should be able to start the receiver")
	defer rcvr.Shutdown(context.Background())

	err = rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.True(t, errors.Is(err, componenterror.ErrAlreadyStarted), "should not start receiver instance twice")
}

func TestCantStopAnInstanceTwice(t *testing.T) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	sink := new(exportertest.SinkTraceExporter)
	rcvr, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: transport,
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.NoError(t, err, "receiver should be created")

	// start pollers
	err = rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "should be able to start receiver")

	pollerStops := make(chan bool)
	go func() {
		err = rcvr.Shutdown(context.Background())
		assert.NoError(t, err, "should be able to shutdown the receiver")
		close(pollerStops)
	}()

	testutil.WaitFor(t, func() bool {
		select {
		case _, open := <-pollerStops:
			return !open
		default:
			return false
		}
	}, "poller is not stopped")

	err = rcvr.Shutdown(context.Background())
	assert.True(t, errors.Is(err, componenterror.ErrAlreadyStopped), "should not stop receiver instance twice")
}

// TODO: Update this test to assert on the format of traces
// once the transformation from X-Ray segments -> OTLP is done.
func TestSegmentsPassedToConsumer(t *testing.T) {
	addr, rcvr, _ := createAndOptionallyStartReceiver(t, true)
	defer rcvr.Shutdown(context.Background())

	// valid header with invalid body (for now this is ok because we haven't
	// implemented the X-Ray segment -> OT format conversion)
	err := writePacket(t, addr, `{"format": "json", "version": 1}`+"\nBody")
	assert.NoError(t, err, "can not write packet in the happy case")

	sink := rcvr.(*xrayReceiver).consumer.(*exportertest.SinkTraceExporter)

	testutil.WaitFor(t, func() bool {
		got := sink.AllTraces()
		if len(got) == 1 {
			return true
		}
		return false
	}, "consumer should eventually get the X-Ray span")
}

func TestIssuesOccurredWhenSplitHeaderBody(t *testing.T) {
	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, true)
	defer rcvr.Shutdown(context.Background())

	err := writePacket(t, addr, "Header\n") // no body
	assert.NoError(t, err, "can not write packet in the no body test case")
	testutil.WaitFor(t, func() bool {
		logs := recordedLogs.All()
		if len(logs) > 0 && strings.Contains(logs[len(logs)-1].Message, "Missing header or segment") {
			return true
		}
		return false
	}, "poller should reject segment")
}

func TestInvalidHeader(t *testing.T) {
	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, true)
	defer rcvr.Shutdown(context.Background())

	randString, _ := uuid.NewRandom()
	// the header (i.e. the portion before \n) is invalid
	err := writePacket(t, addr, randString.String()+"\nBody")
	assert.NoError(t, err, "can not write packet in the invalid header test case")
	testutil.WaitFor(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		if len(logs) > 0 &&
			strings.Contains(lastEntry.Message, "Invalid header") &&
			// assert the invalid header is equal to the random string we passed
			// in previously as the invalid header.
			strings.Compare(string(lastEntry.Context[0].Interface.([]byte)), randString.String()) == 0 {
			return true
		}
		return false
	}, "poller should reject segment")
}

func TestSocketReadIrrecoverableNetError(t *testing.T) {
	_, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, false)
	// close the actual socket because we are going to mock it out below
	rcvr.(*xrayReceiver).udpSock.Close()

	randErrStr, _ := uuid.NewRandom()
	rcvr.(*xrayReceiver).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockNetError{
			mockErrStr: randErrStr.String(),
		},
	}

	err := rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "receiver with mock socket should be started")

	testutil.WaitFor(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errIrrecv *errIrrecoverable
		if len(logs) > 0 &&
			strings.Contains(lastEntry.Message, "irrecoverable socket read error. Exiting poller") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errIrrecv) &&
			strings.Compare(errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error(), randErrStr.String()) == 0 {
			return true
		}
		return false
	}, "poller should exit due to irrecoverable net read error")
}

func TestSocketReadTemporaryNetError(t *testing.T) {
	_, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, false)
	// close the actual socket because we are going to mock it out below
	rcvr.(*xrayReceiver).udpSock.Close()

	randErrStr, _ := uuid.NewRandom()
	rcvr.(*xrayReceiver).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockNetError{
			mockErrStr: randErrStr.String(),
			temporary:  true,
		},
	}

	err := rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "receiver with mock socket should be started")

	testutil.WaitFor(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errRecv *errRecoverable
		if len(logs) > 0 &&
			strings.Contains(lastEntry.Message, "recoverable socket read error") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			strings.Compare(errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error(), randErrStr.String()) == 0 {
			return true
		}
		return false
	}, "poller should encounter net read error")
}

func TestSocketGenericReadError(t *testing.T) {
	_, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, false)
	// close the actual socket because we are going to mock it out below
	rcvr.(*xrayReceiver).udpSock.Close()

	randErrStr, _ := uuid.NewRandom()
	rcvr.(*xrayReceiver).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockGenericErr{
			mockErrStr: randErrStr.String(),
		},
	}

	err := rcvr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "receiver with mock socket should be started")

	testutil.WaitFor(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errRecv *errRecoverable
		if len(logs) > 0 &&
			strings.Contains(lastEntry.Message, "recoverable socket read error") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			strings.Compare(errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error(), randErrStr.String()) == 0 {
			return true
		}
		return false
	}, "poller should encounter generic socket read error")
}

type mockNetError struct {
	mockErrStr string
	temporary  bool
}

func (m *mockNetError) Error() string {
	return m.mockErrStr
}

func (m *mockNetError) Timeout() bool {
	return false
}

func (m *mockNetError) Temporary() bool {
	return m.temporary
}

type mockGenericErr struct {
	mockErrStr string
}

func (m *mockGenericErr) Error() string {
	return m.mockErrStr
}

type mockSocketConn struct {
	expectedOutput []byte
	expectedError  error
	readCount      int
	mu             sync.Mutex
}

func (m *mockSocketConn) Read(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := copy(b, m.expectedOutput)
	if m.readCount > 0 {
		// intentionally slow to prevent a busy loop during any unit tests
		// that involve the poll() function
		time.Sleep(5 * time.Second)
	}
	m.readCount++
	return copied, m.expectedError
}

func (m *mockSocketConn) Close() {}

func createAndOptionallyStartReceiver(t *testing.T, start bool) (string, component.TraceReceiver, *observer.ObservedLogs) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	sink := new(exportertest.SinkTraceExporter)
	logger, recorded := logSetup()
	rcvr, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: transport,
			},
		},
		sink,
		logger,
	)
	assert.NoError(t, err, "receiver should be created")

	if start {
		err = rcvr.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err, "receiver should be started")
	}
	return addr, rcvr, recorded
}

// findAvailableAddress finds an available local address+port and returns it.
// There might be race condition on the address returned by this function if
// there's some other code that grab the address before we can listen on it.
func findAvailableAddress() (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return "", err
	}

	sock, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "", err
	}
	defer sock.Close()
	return sock.LocalAddr().String(), nil
}

func writePacket(t *testing.T, addr, toWrite string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := fmt.Fprintf(conn, toWrite)
	if err != nil {
		return err
	}
	assert.Equal(t, len(toWrite), n, "exunpected number of bytes written")
	return nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.InfoLevel)
	return zap.New(core), recorded
}
