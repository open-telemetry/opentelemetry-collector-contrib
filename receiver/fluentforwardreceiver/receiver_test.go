// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

func setupServer(t *testing.T) (func() net.Conn, *consumertest.LogsSink, *observer.ObservedLogs, context.CancelFunc, receiver.Logs) {
	ctx, cancel := context.WithCancel(t.Context())

	next := new(consumertest.LogsSink)
	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger

	conf := &Config{
		ListenAddress: "127.0.0.1:0",
	}

	receiver, err := newFluentReceiver(set, conf, next)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

	connect := func() net.Conn {
		conn, err := net.Dial("tcp", receiver.(*fluentReceiver).listener.Addr().String())
		require.NoError(t, err)
		return conn
	}

	go func() {
		<-ctx.Done()
		assert.NoError(t, receiver.Shutdown(ctx))
	}()

	return connect, next, logObserver, cancel, receiver
}

func setupServerWithConsumer(t *testing.T, next consumer.Logs, configure func(*fluentReceiver)) (func() net.Conn, *observer.ObservedLogs, context.CancelFunc, receiver.Logs) {
	ctx, cancel := context.WithCancel(t.Context())

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	set := receivertest.NewNopSettings(metadata.Type)
	set.Logger = logger

	conf := &Config{
		ListenAddress: "127.0.0.1:0",
	}

	recv, err := newFluentReceiver(set, conf, next)
	require.NoError(t, err)
	if configure != nil {
		configure(recv.(*fluentReceiver))
	}
	require.NoError(t, recv.Start(ctx, componenttest.NewNopHost()))

	connect := func() net.Conn {
		conn, err := net.Dial("tcp", recv.(*fluentReceiver).listener.Addr().String())
		require.NoError(t, err)
		return conn
	}

	go func() {
		<-ctx.Done()
		assert.NoError(t, recv.Shutdown(ctx))
	}()

	return connect, logObserver, cancel, recv
}

func waitForConnectionClose(t *testing.T, conn net.Conn) {
	one := make([]byte, 1)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err := conn.Read(one)
	// If this is a timeout, then the connection didn't actually close like
	// expected.
	require.Equal(t, io.EOF, err)
}

// Make sure malformed events don't cause panics.
func TestMessageEventConversionMalformed(t *testing.T) {
	connect, _, observedLogs, cancel, _ := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/message-event")

	vulnerableBits := []int{0, 1, 14, 59}

	for _, pos := range vulnerableBits {
		eventBytes[pos]++

		conn := connect()
		n, err := conn.Write(eventBytes)
		require.NoError(t, err)
		require.Len(t, eventBytes, n)

		waitForConnectionClose(t, conn)

		require.Len(t, observedLogs.FilterMessageSnippet("Unexpected").All(), 1)
		_ = observedLogs.TakeAll()
	}
}

func TestMessageEvent(t *testing.T) {
	connect, next, _, cancel, _ := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/message-event")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)
	require.NoError(t, conn.Close())

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, plogtest.CompareLogs(logConstructor(log{
		Timestamp: 1593031012000000000,
		Body:      pcommon.NewValueStr("..."),
		Attributes: map[string]any{
			"container_id":   "b00a67eb645849d6ab38ff8beb4aad035cc7e917bf123c3e9057c7e89fc73d2d",
			"container_name": "/unruffled_cannon",
			"fluent.tag":     "b00a67eb6458",
			"source":         "stdout",
		},
	}), converted[0]))
}

func TestForwardEvent(t *testing.T) {
	connect, next, _, cancel, _ := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/forward-event")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)
	require.NoError(t, conn.Close())

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, plogtest.CompareLogs(logConstructor(
		log{
			Timestamp: 1593032377776693638,
			Body:      pcommon.NewValueEmpty(),
			Attributes: map[string]any{
				"Mem.free":   848908,
				"Mem.total":  7155496,
				"Mem.used":   6306588,
				"Swap.free":  0,
				"Swap.total": 0,
				"Swap.used":  0,
				"fluent.tag": "mem.0",
			},
		},
		log{
			Timestamp: 1593032378756829346,
			Body:      pcommon.NewValueEmpty(),
			Attributes: map[string]any{
				"Mem.free":   848908,
				"Mem.total":  7155496,
				"Mem.used":   6306588,
				"Swap.free":  0,
				"Swap.total": 0,
				"Swap.used":  0,
				"fluent.tag": "mem.0",
			},
		},
	), converted[0]))
}

func TestEventAcknowledgment(t *testing.T) {
	connect, _, logs, cancel, _ := setupServer(t)
	defer func() { fmt.Printf("%v\n", logs.All()) }()
	defer cancel()

	const chunkValue = "abcdef01234576789"

	var b []byte

	// Make a message event with the chunk option
	b = msgp.AppendArrayHeader(b, 4)
	b = msgp.AppendString(b, "my-tag")
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)
	b = msgp.AppendMapStrStr(b, map[string]string{"chunk": chunkValue})

	conn := connect()
	n, err := conn.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	resp := map[string]any{}
	err = msgp.NewReader(conn).ReadMapStrIntf(resp)
	require.NoError(t, err)

	require.Equal(t, chunkValue, resp["ack"])
}

func TestEventAcknowledgmentWaitsForConsumerSuccess(t *testing.T) {
	release := make(chan struct{})
	next := newGatedLogsConsumer(release, nil)
	connect, _, cancel, _ := setupServerWithConsumer(t, next, func(r *fluentReceiver) {
		r.collector.ackWaitTimeout = 5 * time.Second
		r.server.ackWaitTimeout = 5 * time.Second
	})
	defer cancel()

	const chunkValue = "chunk-success"
	conn := connect()
	n, err := conn.Write(makeSampleEventWithChunk("my-tag", chunkValue))
	require.NoError(t, err)
	require.Positive(t, n)

	require.Eventually(t, next.called, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
	require.True(t, isTimeout(err), "expected no ACK before ConsumeLogs returned, got: %v", err)

	close(release)
	resp := readACKResponse(t, conn)
	require.Equal(t, chunkValue, resp["ack"])
}

func TestEventAcknowledgmentDownstreamErrorClosesWithoutACK(t *testing.T) {
	next := newErrLogsConsumer(errors.New("pipeline full"))
	connect, _, cancel, _ := setupServerWithConsumer(t, next, nil)
	defer cancel()

	conn := connect()
	n, err := conn.Write(makeSampleEventWithChunk("my-tag", "chunk-error"))
	require.NoError(t, err)
	require.Positive(t, n)

	require.Eventually(t, next.called, 5*time.Second, 10*time.Millisecond)
	waitForConnectionClose(t, conn)
}

func TestEventAcknowledgmentTimeoutClosesWithoutACK(t *testing.T) {
	release := make(chan struct{})
	next := newGatedLogsConsumer(release, nil)
	connect, _, cancel, _ := setupServerWithConsumer(t, next, func(r *fluentReceiver) {
		r.collector.ackWaitTimeout = 50 * time.Millisecond
		r.server.ackWaitTimeout = 50 * time.Millisecond
	})
	defer cancel()

	conn := connect()
	n, err := conn.Write(makeSampleEventWithChunk("my-tag", "chunk-timeout"))
	require.NoError(t, err)
	require.Positive(t, n)

	require.Eventually(t, next.called, 5*time.Second, 10*time.Millisecond)
	waitForConnectionClose(t, conn)
}

func TestEventAcknowledgmentEnqueueTimeoutClosesWithoutACK(t *testing.T) {
	set := receivertest.NewNopSettings(metadata.Type)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(t, err)
	defer telemetryBuilder.Shutdown()

	eventCh := make(chan eventWithACK)
	srv := newServer(eventCh, 50*time.Millisecond, zap.NewNop(), telemetryBuilder)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	serverDone := make(chan error, 1)
	go func() {
		err := srv.handleConn(t.Context(), serverConn)
		_ = serverConn.Close()
		serverDone <- err
	}()

	writeDone := make(chan error, 1)
	go func() {
		_, err := clientConn.Write(makeSampleEventWithChunk("my-tag", "chunk-enqueue-timeout"))
		writeDone <- err
	}()
	if err := <-writeDone; err != nil {
		require.ErrorIs(t, err, io.ErrClosedPipe)
	}

	select {
	case err := <-serverDone:
		require.ErrorContains(t, err, "timed out waiting to enqueue chunk for acknowledgment")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for server enqueue timeout")
	}
	_, err = clientConn.Read(make([]byte, 1))
	require.True(t, errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF), "expected closed connection, got: %v", err)
}

func TestEventAcknowledgmentBoundedPendingWaiters(t *testing.T) {
	release := make(chan struct{})
	next := newGatedLogsConsumer(release, nil)
	connect, _, cancel, _ := setupServerWithConsumer(t, next, func(r *fluentReceiver) {
		r.collector.ackWaitTimeout = 5 * time.Second
		r.server.ackWaitTimeout = 5 * time.Second
		r.server.ackWaiters = make(chan struct{}, 1)
	})
	defer cancel()

	firstConn := connect()
	n, err := firstConn.Write(makeSampleEventWithChunk("my-tag", "chunk-blocked"))
	require.NoError(t, err)
	require.Positive(t, n)
	require.Eventually(t, next.called, 5*time.Second, 10*time.Millisecond)

	secondConn := connect()
	n, err = secondConn.Write(makeSampleEventWithChunk("my-tag", "chunk-rejected"))
	require.NoError(t, err)
	require.Positive(t, n)
	waitForConnectionClose(t, secondConn)

	close(release)
	resp := readACKResponse(t, firstConn)
	require.Equal(t, "chunk-blocked", resp["ack"])
}

func TestForwardEventAcknowledgment(t *testing.T) {
	connect, _, _, cancel, _ := setupServer(t)
	defer cancel()

	const chunkValue = "chunk-forward"
	conn := connect()
	n, err := conn.Write(makeSampleForwardEventWithChunk("my-tag", chunkValue))
	require.NoError(t, err)
	require.Positive(t, n)

	resp := readACKResponse(t, conn)
	require.Equal(t, chunkValue, resp["ack"])
}

func TestPackedForwardEventAcknowledgment(t *testing.T) {
	connect, _, _, cancel, _ := setupServer(t)
	defer cancel()

	const chunkValue = "chunk-packed-forward"
	conn := connect()
	n, err := conn.Write(makeSamplePackedForwardEventWithChunk("my-tag", chunkValue))
	require.NoError(t, err)
	require.Positive(t, n)

	resp := readACKResponse(t, conn)
	require.Equal(t, chunkValue, resp["ack"])
}

func TestEventAcknowledgmentShutdownWhileWaiting(t *testing.T) {
	release := make(chan struct{})
	next := newGatedLogsConsumer(release, nil)
	connect, _, cancel, recv := setupServerWithConsumer(t, next, func(r *fluentReceiver) {
		r.collector.ackWaitTimeout = 5 * time.Second
		r.server.ackWaitTimeout = 5 * time.Second
	})
	defer cancel()
	defer close(release)

	conn := connect()
	n, err := conn.Write(makeSampleEventWithChunk("my-tag", "chunk-shutdown"))
	require.NoError(t, err)
	require.Positive(t, n)
	require.Eventually(t, next.called, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, recv.Shutdown(t.Context()))
	waitForConnectionClose(t, conn)
}

func TestForwardPackedEvent(t *testing.T) {
	connect, next, _, cancel, _ := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/forward-packed")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)
	require.NoError(t, conn.Close())

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, plogtest.CompareLogs(logConstructor(
		log{
			Timestamp: 1593032517024597622,
			Body:      pcommon.NewValueStr("starting fluentd worker pid=17 ppid=7 worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"pid":        17,
				"ppid":       7,
				"worker":     0,
			},
		},
		log{
			Timestamp: 1593032517028573686,
			Body:      pcommon.NewValueStr("delayed_commit_timeout is overwritten by ack_response_timeout"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
			},
		},
		log{
			Timestamp: 1593032517028815948,
			Body:      pcommon.NewValueStr("following tail of /var/log/kern.log"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
			},
		},
		log{
			Timestamp: 1593032517031174229,
			Body:      pcommon.NewValueStr("fluentd worker is now running worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"worker":     0,
			},
		},
		log{
			Timestamp: 1593032522187382822,
			Body:      pcommon.NewValueStr("fluentd worker is now stopping worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"worker":     0,
			},
		},
	), converted[0]))
}

func TestForwardPackedCompressedEvent(t *testing.T) {
	connect, next, _, cancel, _ := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/forward-packed-compressed")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)
	require.NoError(t, conn.Close())

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, plogtest.CompareLogs(logConstructor(
		log{
			Timestamp: 1593032426012197420,
			Body:      pcommon.NewValueStr("starting fluentd worker pid=17 ppid=7 worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"pid":        17,
				"ppid":       7,
				"worker":     0,
			},
		},
		log{
			Timestamp: 1593032426013724933,
			Body:      pcommon.NewValueStr("delayed_commit_timeout is overwritten by ack_response_timeout"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
			},
		},
		log{
			Timestamp: 1593032426020510455,
			Body:      pcommon.NewValueStr("following tail of /var/log/kern.log"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
			},
		},
		log{
			Timestamp: 1593032426024346580,
			Body:      pcommon.NewValueStr("fluentd worker is now running worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"worker":     0,
			},
		},
		log{
			Timestamp: 1593032434346935532,
			Body:      pcommon.NewValueStr("fluentd worker is now stopping worker=0"),
			Attributes: map[string]any{
				"fluent.tag": "fluent.info",
				"worker":     0,
			},
		},
	), converted[0]))
}

func TestUnixEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	next := new(consumertest.LogsSink)

	tmpdir := t.TempDir()

	conf := &Config{
		ListenAddress: "unix://" + filepath.Join(tmpdir, "fluent.sock"),
	}

	receiver, err := newFluentReceiver(receivertest.NewNopSettings(metadata.Type), conf, next)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	defer func() { require.NoError(t, receiver.Shutdown(ctx)) }()

	conn, err := net.Dial("unix", receiver.(*fluentReceiver).listener.Addr().String())
	require.NoError(t, err)

	n, err := conn.Write(parseHexDump("testdata/message-event"))
	require.NoError(t, err)
	require.Positive(t, n)

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)
}

func makeSampleEvent(tag string) []byte {
	var b []byte

	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, tag)
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)
	return b
}

func makeSampleEventWithChunk(tag, chunk string) []byte {
	var b []byte

	b = msgp.AppendArrayHeader(b, 4)
	b = msgp.AppendString(b, tag)
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)
	b = msgp.AppendMapStrStr(b, map[string]string{"chunk": chunk})
	return b
}

func makeSampleForwardEventWithChunk(tag, chunk string) []byte {
	var b []byte

	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, tag)
	b = msgp.AppendArrayHeader(b, 1)
	b = appendSampleEntry(b)
	b = msgp.AppendMapStrStr(b, map[string]string{"chunk": chunk})
	return b
}

func makeSamplePackedForwardEventWithChunk(tag, chunk string) []byte {
	var entries []byte
	entries = appendSampleEntry(entries)

	var b []byte
	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, tag)
	b = msgp.AppendBytes(b, entries)
	b = msgp.AppendMapStrStr(b, map[string]string{"chunk": chunk})
	return b
}

func appendSampleEntry(b []byte) []byte {
	b = msgp.AppendArrayHeader(b, 2)
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)
	return b
}

func TestHighVolume(t *testing.T) {
	connect, next, _, cancel, _ := setupServer(t)
	defer cancel()

	const totalRoutines = 8
	const totalMessagesPerRoutine = 1000

	var wg sync.WaitGroup
	for i := range totalRoutines {
		wg.Add(1)
		go func(num int) {
			conn := connect()
			for j := range totalMessagesPerRoutine {
				eventBytes := makeSampleEvent(fmt.Sprintf("tag-%d-%d", num, j))
				n, err := conn.Write(eventBytes)
				assert.NoError(t, err)
				assert.Equal(t, len(eventBytes), n)
			}
			assert.NoError(t, conn.Close())
			wg.Done()
		}(i)
	}

	wg.Wait()

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()

		var total int
		for i := range converted {
			total += converted[i].LogRecordCount()
		}

		return total == totalRoutines*totalMessagesPerRoutine
	}, 10*time.Second, 100*time.Millisecond)
}

// TestReceiverShutdownRefusesNewConnections verifies the graceful shutdown of a receiver, ensuring no new connections can be established afterward.
func TestReceiverShutdownRefusesNewConnections(t *testing.T) {
	connect, next, _, cancel, receiver := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/message-event")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)
	require.NoError(t, conn.Close())

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// shutdown the receiver
	require.NoError(t, receiver.Shutdown(t.Context()))

	// New connection will be refused
	_, err = net.Dial("tcp", receiver.(*fluentReceiver).listener.Addr().String())
	require.Error(t, err)
}

// TestReceiverShutdownClosesExistingConnections verifies the graceful shutdown of a receiver, ensuring existing connections will be closed.
func TestReceiverShutdownClosesExistingConnections(t *testing.T) {
	connect, next, _, cancel, receiver := setupServer(t)
	defer cancel()

	eventBytes := parseHexDump("testdata/message-event")

	conn := connect()
	n, err := conn.Write(eventBytes)
	require.NoError(t, err)
	require.Equal(t, len(eventBytes), n)

	var converted []plog.Logs
	require.Eventually(t, func() bool {
		converted = next.AllLogs()
		return len(converted) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// shutdown the receiver
	require.NoError(t, receiver.Shutdown(t.Context()))

	// Existing connection will be refused
	require.Eventually(t, func() bool {
		_, err := conn.Write(eventBytes)
		return err != nil
	}, 5*time.Second, 1*time.Second)
}

type gatedLogsConsumer struct {
	release chan struct{}
	err     error
	seen    atomic.Bool
}

func newGatedLogsConsumer(release chan struct{}, err error) *gatedLogsConsumer {
	return &gatedLogsConsumer{
		release: release,
		err:     err,
	}
}

func (c *gatedLogsConsumer) ConsumeLogs(ctx context.Context, _ plog.Logs) error {
	c.seen.Store(true)
	select {
	case <-c.release:
		return c.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (*gatedLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *gatedLogsConsumer) called() bool {
	return c.seen.Load()
}

type errLogsConsumer struct {
	err  error
	seen atomic.Bool
}

func newErrLogsConsumer(err error) *errLogsConsumer {
	return &errLogsConsumer{err: err}
}

func (c *errLogsConsumer) ConsumeLogs(context.Context, plog.Logs) error {
	c.seen.Store(true)
	return c.err
}

func (*errLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *errLogsConsumer) called() bool {
	return c.seen.Load()
}

func readACKResponse(t *testing.T, conn net.Conn) map[string]any {
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	resp := map[string]any{}
	require.NoError(t, msgp.NewReader(conn).ReadMapStrIntf(resp))
	return resp
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
