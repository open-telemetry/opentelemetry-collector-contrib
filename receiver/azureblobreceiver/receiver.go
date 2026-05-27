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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"
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
	logsUnmarshaler    plog.Unmarshaler
	tracesUnmarshaler  ptrace.Unmarshaler
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

	logsContext := b.obsrecv.StartLogsOp(ctx)

	logs, err := b.logsUnmarshaler.UnmarshalLogs(data)
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

	tracesContext := b.obsrecv.StartTracesOp(ctx)

	traces, err := b.tracesUnmarshaler.UnmarshalTraces(data)
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

	var logsUnmarshaler plog.Unmarshaler
	switch logsEncoding {
	case EncodingOTLPJSON:
		logsUnmarshaler = &plog.JSONUnmarshaler{}
	case EncodingOTLPProto:
		logsUnmarshaler = &plog.ProtoUnmarshaler{}
	case EncodingAzureResourceLogs:
		logsUnmarshaler = &azurelogs.ResourceLogsUnmarshaler{
			Version: set.BuildInfo.Version,
			Logger:  set.Logger,
		}
	default:
		return nil, fmt.Errorf("logs.encoding %q is not supported; supported values: [%v, %v, %v]", logsEncoding, EncodingOTLPJSON, EncodingOTLPProto, EncodingAzureResourceLogs)
	}

	var tracesUnmarshaler ptrace.Unmarshaler
	switch tracesEncoding {
	case EncodingOTLPJSON:
		tracesUnmarshaler = &ptrace.JSONUnmarshaler{}
	case EncodingOTLPProto:
		tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
	default:
		return nil, fmt.Errorf("traces.encoding %q is not supported; supported values: [%v, %v]", tracesEncoding, EncodingOTLPJSON, EncodingOTLPProto)
	}

	blobReceiver := &blobReceiver{
		blobEventHandler:  eventHandler,
		logger:            set.Logger,
		logsUnmarshaler:   logsUnmarshaler,
		tracesUnmarshaler: tracesUnmarshaler,
		obsrecv:           obsrecv,
	}

	return blobReceiver, nil
}
