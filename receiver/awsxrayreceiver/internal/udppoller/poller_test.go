// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udppoller

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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	internalErr "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
)

func TestNonUDPTransport(t *testing.T) {
	_, err := New(
		&Config{
			Transport:          "tcp",
			NumOfPollerToStart: 2,
		},
		receivertest.NewNopCreateSettings(),
	)
	assert.EqualError(t, err,
		"X-Ray receiver only supports ingesting spans through UDP, provided: tcp")
}

func TestInvalidEndpoint(t *testing.T) {
	_, err := New(
		&Config{
			Endpoint:           "invalidAddr",
			Transport:          "udp",
			NumOfPollerToStart: 2,
		},
		receivertest.NewNopCreateSettings(),
	)
	assert.EqualError(t, err, "address invalidAddr: missing port in address")
}

func TestUDPPortUnavailable(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	defer sock.Close()
	address := sock.LocalAddr().String()

	_, err = New(
		&Config{
			Transport:          Transport,
			Endpoint:           address,
			NumOfPollerToStart: 2,
		},
		receivertest.NewNopCreateSettings(),
	)

	assert.Error(t, err, "should have failed to create a new receiver")
	assert.True(t,
		strings.Contains(err.Error(), "address already in use") || strings.Contains(err.Error(), "Only one usage of each socket address"),
		"error message should complain about address in-use")
}

func TestCloseStopsPoller(t *testing.T) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	p, err := New(
		&Config{
			Transport:          Transport,
			Endpoint:           addr,
			NumOfPollerToStart: 2,
		},
		receivertest.NewNopCreateSettings(),
	)
	assert.NoError(t, err, "poller should be created")

	// start pollers
	segChan := p.SegmentsChan()
	p.Start(context.Background())

	err = p.Close()
	assert.NoError(t, err, "should be able to close the poller")

	assert.Eventuallyf(t, func() bool {
		select {
		case _, open := <-segChan:
			return !open
		default:
			return false
		}
	}, 10*time.Second, 5*time.Millisecond, "output channel should be closed")

	err = p.(*poller).udpSock.Close()
	assert.Error(t, err, "a socket should not be closed twice")
}

func TestSuccessfullyPollPacket(t *testing.T) {
	receiverID := component.NewID("TestSuccessfullyPollPacket")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, p, _ := createAndOptionallyStartPoller(t, true, tt.ToReceiverCreateSettings())
	defer p.Close()

	randString, _ := uuid.NewRandom()
	rawData := []byte(`{"format": "json", "version": 1}` + "\n" + randString.String())
	err = writePacket(t, addr, string(rawData))
	assert.NoError(t, err, "can not write packet in the TestSuccessfullyPollPacket case")

	assert.Eventuallyf(t, func() bool {
		select {
		case seg, open := <-p.(*poller).segChan:
			obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
				ReceiverID:             receiverID,
				Transport:              Transport,
				ReceiverCreateSettings: tt.ToReceiverCreateSettings(),
			})
			require.NoError(t, err)
			ctx := obsrecv.StartMetricsOp(seg.Ctx)
			obsrecv.EndTracesOp(ctx, metadata.Type, 1, nil)
			return open && randString.String() == string(seg.Payload)
		default:
			return false
		}
	}, 10*time.Second, 5*time.Millisecond, "poller should return parsed segment")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 2, 0))
}

func TestIncompletePacketNoSeparator(t *testing.T) {
	receiverID := component.NewID("TestIncompletePacketNoSeparator")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, p, recordedLogs := createAndOptionallyStartPoller(t, true, tt.ToReceiverCreateSettings())
	defer p.Close()

	rawData := []byte(`{"format": "json", "version": 1}`) // no separator
	err = writePacket(t, addr, string(rawData))
	assert.NoError(t, err, "can not write packet in the TestIncompletePacketNoSeparator case")
	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errRecv *internalErr.ErrRecoverable

		return strings.Contains(lastEntry.Message, "Failed to split segment header and body") &&
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			strings.Compare(
				errors.Unwrap(
					lastEntry.Context[0].Interface.(error)).Error(),
				fmt.Sprintf("unable to split incoming data as header and segment, incoming bytes: %v", rawData)) == 0
	}, 10*time.Second, 5*time.Millisecond, "poller should reject segment")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestIncompletePacketNoBody(t *testing.T) {
	receiverID := component.NewID("TestIncompletePacketNoBody")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, p, recordedLogs := createAndOptionallyStartPoller(t, true, tt.ToReceiverCreateSettings())
	defer p.Close()

	rawData := []byte(`{"format": "json", "version": 1}` + "\n") // no body
	err = writePacket(t, addr, string(rawData))
	assert.NoError(t, err, "can not write packet in the TestIncompletePacketNoBody case")
	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		return strings.Contains(lastEntry.Message, "Missing body") &&
			lastEntry.Context[0].String == "json" &&
			lastEntry.Context[1].Integer == 1
	}, 10*time.Second, 5*time.Millisecond, "poller should log missing body")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestNonJsonHeader(t *testing.T) {
	receiverID := component.NewID("TestNonJsonHeader")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, p, recordedLogs := createAndOptionallyStartPoller(t, true, tt.ToReceiverCreateSettings())
	defer p.Close()

	// the header (i.e. the portion before \n) is invalid
	err = writePacket(t, addr, "nonJson\nBody")
	assert.NoError(t, err, "can not write packet in the TestNonJsonHeader case")
	assert.Eventuallyf(t, func() bool {
		var errRecv *internalErr.ErrRecoverable
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]

		return lastEntry.Message == "Failed to split segment header and body" &&
			// assert the invalid header is equal to the random string we passed
			// in previously as the invalid header.
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			strings.Contains(lastEntry.Context[0].Interface.(error).Error(),
				"invalid character 'o'")
	}, 10*time.Second, 5*time.Millisecond, "poller should reject segment")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestJsonInvalidHeader(t *testing.T) {
	receiverID := component.NewID("TestJsonInvalidHeader")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, p, recordedLogs := createAndOptionallyStartPoller(t, true, tt.ToReceiverCreateSettings())
	defer p.Close()

	randString, _ := uuid.NewRandom()
	// the header (i.e. the portion before \n) is invalid
	err = writePacket(t, addr,
		fmt.Sprintf(`{"format": "%s", "version": 1}`, randString.String())+"\nBody")
	assert.NoError(t, err, "can not write packet in the TestJsonInvalidHeader case")
	assert.Eventuallyf(t, func() bool {
		var errRecv *internalErr.ErrRecoverable
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		return lastEntry.Message == "Failed to split segment header and body" &&
			// assert the invalid header is equal to the random string we passed
			// in previously as the invalid header.
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			errors.Unwrap(
				lastEntry.Context[0].Interface.(error)).Error() == fmt.Sprintf(
				"invalid header %+v", tracesegment.Header{
					Format:  randString.String(),
					Version: 1,
				},
			)
	}, 10*time.Second, 5*time.Millisecond, "poller should reject segment")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestSocketReadIrrecoverableNetError(t *testing.T) {
	receiverID := component.NewID("TestSocketReadIrrecoverableNetError")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	_, p, recordedLogs := createAndOptionallyStartPoller(t, false, tt.ToReceiverCreateSettings())
	// close the actual socket because we are going to mock it out below
	p.(*poller).udpSock.Close()

	// replace actual socket with the mock
	randErrStr, _ := uuid.NewRandom()
	p.(*poller).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockNetError{
			mockErrStr: randErrStr.String(),
		},
	}

	p.Start(context.Background())

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errIrrecv *internalErr.ErrIrrecoverable
		return strings.Contains(lastEntry.Message, "Irrecoverable socket read error. Exiting poller") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errIrrecv) &&
			errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error() == randErrStr.String()
	}, 10*time.Second, 5*time.Millisecond, "poller should exit due to irrecoverable net read error")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestSocketReadTimeOutNetError(t *testing.T) {
	receiverID := component.NewID("TestSocketReadTimeOutNetError")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	_, p, recordedLogs := createAndOptionallyStartPoller(t, false, tt.ToReceiverCreateSettings())
	// close the actual socket because we are going to mock it out below
	p.(*poller).udpSock.Close()

	// again replace the socket with a mock
	randErrStr, _ := uuid.NewRandom()
	p.(*poller).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockNetError{
			mockErrStr: randErrStr.String(),
			timeout:    true,
		},
	}

	p.Start(context.Background())

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errRecv *internalErr.ErrRecoverable
		return strings.Contains(lastEntry.Message, "Recoverable socket read error") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error() == randErrStr.String()
	}, 10*time.Second, 5*time.Millisecond, "poller should encounter net read error")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

func TestSocketGenericReadError(t *testing.T) {
	receiverID := component.NewID("TestSocketGenericReadError")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	_, p, recordedLogs := createAndOptionallyStartPoller(t, false, tt.ToReceiverCreateSettings())
	// close the actual socket because we are going to mock it out below
	p.(*poller).udpSock.Close()

	randErrStr, _ := uuid.NewRandom()
	p.(*poller).udpSock = &mockSocketConn{
		expectedOutput: []byte("dontCare"),
		expectedError: &mockGenericErr{
			mockErrStr: randErrStr.String(),
		},
	}

	p.Start(context.Background())

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		lastEntry := logs[len(logs)-1]
		var errRecv *internalErr.ErrRecoverable
		return strings.Contains(lastEntry.Message, "Recoverable socket read error") &&
			lastEntry.Context[0].Type == zapcore.ErrorType &&
			errors.As(lastEntry.Context[0].Interface.(error), &errRecv) &&
			errors.Unwrap(lastEntry.Context[0].Interface.(error)).Error() == randErrStr.String()
	}, 10*time.Second, 5*time.Millisecond, "poller should encounter generic socket read error")

	assert.NoError(t, tt.CheckReceiverTraces(Transport, 0, 1))
}

type mockNetError struct {
	mockErrStr string
	timeout    bool
}

func (m *mockNetError) Error() string {
	return m.mockErrStr
}

func (m *mockNetError) Timeout() bool {
	return m.timeout
}

func (m *mockNetError) Temporary() bool {
	return false
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

func (m *mockSocketConn) Close() error { return nil }

func createAndOptionallyStartPoller(
	t *testing.T,
	start bool,
	set receiver.CreateSettings) (string, Poller, *observer.ObservedLogs) {
	addr, err := findAvailableAddress()
	assert.NoError(t, err, "there should be address available")

	logger, recorded := logSetup()
	set.Logger = logger
	poller, err := New(&Config{
		Transport:          Transport,
		Endpoint:           addr,
		NumOfPollerToStart: 2,
	}, set)
	assert.NoError(t, err, "receiver should be created")

	if start {
		poller.Start(context.Background())
	}
	return addr, poller, recorded
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

	n, err := fmt.Fprint(conn, toWrite)
	if err != nil {
		return err
	}
	assert.Equal(t, len(toWrite), n, "unexpected number of bytes written")
	return nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.InfoLevel)
	return zap.New(core), recorded
}
