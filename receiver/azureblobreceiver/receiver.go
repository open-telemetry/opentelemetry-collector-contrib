// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

type logsDataConsumer interface {
	consumeLogs(ctx context.Context, data []byte) error
	setNextLogsConsumer(nextLogsConsumer consumer.Logs)
}

type tracesDataConsumer interface {
	consumeTraces(ctx context.Context, data []byte) error
	setNextTracesConsumer(nextracesConsumer consumer.Traces)
}

type blobReceiver struct {
	blobEventHandler   eventHandler
	logger             *zap.Logger
	logsEncoding       string
	tracesEncoding     string
	logsUnmarshalers   map[string]plog.Unmarshaler
	tracesUnmarshalers map[string]ptrace.Unmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *receiverhelper.ObsReport
}

func (b *blobReceiver) Start(ctx context.Context, _ component.Host) error {
	err := b.blobEventHandler.run(ctx)

	return err
}

func (b *blobReceiver) Shutdown(ctx context.Context) error {
	return b.blobEventHandler.close(ctx)
}

func (b *blobReceiver) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	b.nextLogsConsumer = nextLogsConsumer
	b.blobEventHandler.setLogsDataConsumer(b)
}

func (b *blobReceiver) setNextTracesConsumer(nextTracesConsumer consumer.Traces) {
	b.nextTracesConsumer = nextTracesConsumer
	b.blobEventHandler.setTracesDataConsumer(b)
}

func (b *blobReceiver) consumeLogs(ctx context.Context, data []byte) error {
	if b.nextLogsConsumer == nil {
		return nil
	}

	unmarshaler, ok := b.logsUnmarshalers[b.logsEncoding]
	if !ok {
		return fmt.Errorf("unsupported logs encoding %q", b.logsEncoding)
	}

	logsContext := b.obsrecv.StartLogsOp(ctx)

	logs, err := unmarshaler.UnmarshalLogs(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	err = b.nextLogsConsumer.ConsumeLogs(logsContext, logs)

	b.obsrecv.EndLogsOp(logsContext, metadata.Type.String(), 1, err)

	return err
}

func (b *blobReceiver) consumeTraces(ctx context.Context, data []byte) error {
	if b.nextTracesConsumer == nil {
		return nil
	}

	unmarshaler, ok := b.tracesUnmarshalers[b.tracesEncoding]
	if !ok {
		return fmt.Errorf("unsupported traces encoding %q", b.tracesEncoding)
	}

	tracesContext := b.obsrecv.StartTracesOp(ctx)

	traces, err := unmarshaler.UnmarshalTraces(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal traces: %w", err)
	}

	err = b.nextTracesConsumer.ConsumeTraces(tracesContext, traces)

	b.obsrecv.EndTracesOp(tracesContext, metadata.Type.String(), 1, err)

	return err
}

// Returns a new instance of the log receiver
func newReceiver(set receiver.Settings, eventHandler eventHandler, logsEncoding, tracesEncoding string) (component.Component, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "event",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	blobReceiver := &blobReceiver{
		blobEventHandler: eventHandler,
		logger:           set.Logger,
		logsEncoding:     logsEncoding,
		tracesEncoding:   tracesEncoding,
		logsUnmarshalers: map[string]plog.Unmarshaler{
			EncodingOTLPJSON:  &plog.JSONUnmarshaler{},
			EncodingOTLPProto: &plog.ProtoUnmarshaler{},
		},
		tracesUnmarshalers: map[string]ptrace.Unmarshaler{
			EncodingOTLPJSON:  &ptrace.JSONUnmarshaler{},
			EncodingOTLPProto: &ptrace.ProtoUnmarshaler{},
		},
		obsrecv: obsrecv,
	}

	return blobReceiver, nil
}
