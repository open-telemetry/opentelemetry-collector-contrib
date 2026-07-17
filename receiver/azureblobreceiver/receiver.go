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
	logsUnmarshaler    plog.Unmarshaler
	tracesUnmarshaler  ptrace.Unmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *receiverhelper.ObsReport
}

func (b *blobReceiver) Start(ctx context.Context, host component.Host) error {
	// Unmarshalers are resolved here rather than at construction because
	// extension-based encodings are only available once the host's extensions
	// have been started.
	if err := b.resolveUnmarshalers(host); err != nil {
		return err
	}

	return b.blobEventHandler.run(ctx)
}

// resolveUnmarshalers resolves the configured logs and traces encodings into
// unmarshalers, looking up encoding extensions from the host as needed.
func (b *blobReceiver) resolveUnmarshalers(host component.Host) error {
	var err error
	if b.logsUnmarshaler, err = newLogsUnmarshaler(b.logsEncoding, host); err != nil {
		return err
	}
	if b.tracesUnmarshaler, err = newTracesUnmarshaler(b.tracesEncoding, host); err != nil {
		return err
	}
	return nil
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

	return &blobReceiver{
		blobEventHandler: eventHandler,
		logger:           set.Logger,
		logsEncoding:     logsEncoding,
		tracesEncoding:   tracesEncoding,
		obsrecv:          obsrecv,
	}, nil
}

// newLogsUnmarshaler resolves encoding into a plog.Unmarshaler. Built-in
// encodings are handled directly; any other value is treated as the ID of an
// encoding extension looked up from the host.
func newLogsUnmarshaler(encoding string, host component.Host) (plog.Unmarshaler, error) {
	switch encoding {
	case EncodingOTLPJSON:
		return &plog.JSONUnmarshaler{}, nil
	case EncodingOTLPProto:
		return &plog.ProtoUnmarshaler{}, nil
	}
	ext, err := encodingExtension(encoding, host)
	if err != nil {
		return nil, fmt.Errorf("logs.%w", err)
	}
	unmarshaler, ok := ext.(plog.Unmarshaler)
	if !ok {
		return nil, fmt.Errorf("logs.encoding extension %q is not a logs unmarshaler", encoding)
	}
	return unmarshaler, nil
}

// newTracesUnmarshaler resolves encoding into a ptrace.Unmarshaler. Built-in
// encodings are handled directly; any other value is treated as the ID of an
// encoding extension looked up from the host.
func newTracesUnmarshaler(encoding string, host component.Host) (ptrace.Unmarshaler, error) {
	switch encoding {
	case EncodingOTLPJSON:
		return &ptrace.JSONUnmarshaler{}, nil
	case EncodingOTLPProto:
		return &ptrace.ProtoUnmarshaler{}, nil
	}
	ext, err := encodingExtension(encoding, host)
	if err != nil {
		return nil, fmt.Errorf("traces.%w", err)
	}
	unmarshaler, ok := ext.(ptrace.Unmarshaler)
	if !ok {
		return nil, fmt.Errorf("traces.encoding extension %q is not a traces unmarshaler", encoding)
	}
	return unmarshaler, nil
}

// encodingExtension resolves the encoding extension referenced by encoding from
// the host's extensions.
func encodingExtension(encoding string, host component.Host) (component.Component, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(encoding)); err != nil {
		return nil, fmt.Errorf("encoding %q is not a valid encoding extension ID: %w", encoding, err)
	}
	ext, ok := host.GetExtensions()[id]
	if !ok {
		return nil, fmt.Errorf("encoding extension %q not found", id)
	}
	return ext, nil
}
