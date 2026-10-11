// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

func TestCollectorCompletesACKsForBatchedEventsAfterConsume(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventCh := make(chan eventWithACK, 2)
	ackCh1 := make(chan error, 1)
	ackCh2 := make(chan error, 1)
	eventCh <- eventWithACK{event: newTestEvent("log1"), ackCh: ackCh1}
	eventCh <- eventWithACK{event: newTestEvent("log2"), ackCh: ackCh2}

	release := make(chan struct{})
	next := &capturingGatedLogsConsumer{
		release: release,
		started: make(chan plog.Logs, 1),
	}

	set := receivertest.NewNopSettings(metadata.Type)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "tcp",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(t, err)
	defer telemetryBuilder.Shutdown()

	c := newCollector(eventCh, next, time.Second, zap.NewNop(), obsrecv, telemetryBuilder)
	c.Start(ctx)

	var consumed plog.Logs
	require.Eventually(t, func() bool {
		select {
		case consumed = <-next.started:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, 2, consumed.LogRecordCount())

	assertNoACKResult(t, ackCh1)
	assertNoACKResult(t, ackCh2)

	close(release)
	require.NoError(t, receiveACKResult(t, ackCh1))
	require.NoError(t, receiveACKResult(t, ackCh2))
}

func TestCollectorCompletesACKsForBatchedEventsAfterConsumeError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	eventCh := make(chan eventWithACK, 2)
	ackCh1 := make(chan error, 1)
	ackCh2 := make(chan error, 1)
	eventCh <- eventWithACK{event: newTestEvent("log1"), ackCh: ackCh1}
	eventCh <- eventWithACK{event: newTestEvent("log2"), ackCh: ackCh2}

	expectedErr := errors.New("downstream unavailable")
	next := newErrLogsConsumer(expectedErr)

	set := receivertest.NewNopSettings(metadata.Type)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "tcp",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(t, err)
	defer telemetryBuilder.Shutdown()

	c := newCollector(eventCh, next, time.Second, zap.NewNop(), obsrecv, telemetryBuilder)
	c.Start(ctx)

	require.ErrorIs(t, receiveACKResult(t, ackCh1), expectedErr)
	require.ErrorIs(t, receiveACKResult(t, ackCh2), expectedErr)
}

func assertNoACKResult(t *testing.T, ackCh <-chan error) {
	select {
	case err := <-ackCh:
		require.Failf(t, "unexpected ACK result", "received %v before ConsumeLogs returned", err)
	default:
	}
}

func receiveACKResult(t *testing.T, ackCh <-chan error) error {
	select {
	case err := <-ackCh:
		return err
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for ACK result")
		return nil
	}
}

type capturingGatedLogsConsumer struct {
	release chan struct{}
	started chan plog.Logs
}

func (c *capturingGatedLogsConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	captured := plog.NewLogs()
	logs.CopyTo(captured)
	c.started <- captured
	select {
	case <-c.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (*capturingGatedLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

type testEvent struct {
	logs plog.LogRecordSlice
}

func newTestEvent(body string) *testEvent {
	logs := plog.NewLogRecordSlice()
	logs.AppendEmpty().Body().SetStr(body)
	return &testEvent{logs: logs}
}

func (*testEvent) DecodeMsg(*msgp.Reader) error {
	return nil
}

func (e *testEvent) LogRecords() plog.LogRecordSlice {
	return e.logs
}

func (*testEvent) Chunk() string {
	return ""
}

func (*testEvent) Compressed() string {
	return ""
}
