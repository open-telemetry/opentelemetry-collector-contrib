// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

type logsDataConsumer interface {
	consumeLogsJSON(ctx context.Context, json []byte) error
	setNextLogsConsumer(nextLogsConsumer consumer.Logs)
}

type tracesDataConsumer interface {
	consumeTracesJSON(ctx context.Context, json []byte) error
	setNextTracesConsumer(nextracesConsumer consumer.Traces)
}

type blobReceiver struct {
	blobEventHandler   blobEventHandler
	logger             *zap.Logger
	logsUnmarshaler    plog.Unmarshaler
	tracesUnmarshaler  ptrace.Unmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *obsreport.Receiver
}

func (b *blobReceiver) Start(ctx context.Context, host component.Host) error {
	err := b.blobEventHandler.run(ctx)

	return err
}

func (b *blobReceiver) Shutdown(ctx context.Context) error {
	return b.blobEventHandler.close(ctx)
}

func (b *blobReceiver) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	b.nextLogsConsumer = nextLogsConsumer
}

func (b *blobReceiver) setNextTracesConsumer(nextTracesConsumer consumer.Traces) {
	b.nextTracesConsumer = nextTracesConsumer
}

func (b *blobReceiver) consumeLogsJSON(ctx context.Context, json []byte) error {

	if b.nextLogsConsumer == nil {
		return nil
	}

	logsContext := b.obsrecv.StartLogsOp(ctx)

	logs, err := b.logsUnmarshaler.UnmarshalLogs(json)
	if err != nil {
		return fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	err = b.nextLogsConsumer.ConsumeLogs(logsContext, logs)

	b.obsrecv.EndLogsOp(logsContext, metadata.Type, 1, err)

	return err
}

func (b *blobReceiver) consumeTracesJSON(ctx context.Context, json []byte) error {
	if b.nextTracesConsumer == nil {
		return nil
	}

	tracesContext := b.obsrecv.StartTracesOp(ctx)

	traces, err := b.tracesUnmarshaler.UnmarshalTraces(json)
	if err != nil {
		return fmt.Errorf("failed to unmarshal traces: %w", err)
	}

	err = b.nextTracesConsumer.ConsumeTraces(tracesContext, traces)

	b.obsrecv.EndTracesOp(tracesContext, metadata.Type, 1, err)

	return err
}

// Returns a new instance of the log receiver
func newReceiver(set receiver.CreateSettings, blobEventHandler blobEventHandler) (component.Component, error) {
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              "event",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	blobReceiver := &blobReceiver{
		blobEventHandler:  blobEventHandler,
		logger:            set.Logger,
		logsUnmarshaler:   &plog.JSONUnmarshaler{},
		tracesUnmarshaler: &ptrace.JSONUnmarshaler{},
		obsrecv:           obsrecv,
	}

	blobEventHandler.setLogsDataConsumer(blobReceiver)
	blobEventHandler.setTracesDataConsumer(blobReceiver)

	return blobReceiver, nil
}
