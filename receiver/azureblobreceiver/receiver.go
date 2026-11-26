// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

type logsDataConsumer interface {
	consumeLogs(ctx context.Context, data io.Reader) error
	setNextLogsConsumer(nextLogsConsumer consumer.Logs)
}

type tracesDataConsumer interface {
	consumeTraces(ctx context.Context, data io.Reader) error
	setNextTracesConsumer(nextracesConsumer consumer.Traces)
}

type blobReceiver struct {
	blobEventHandler   eventHandler
	logger             *zap.Logger
	logsEncoding       string
	tracesEncoding     string
	compression        string
	logsDecoder        encoding.LogsDecoderFactory
	tracesUnmarshaler  ptrace.Unmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *receiverhelper.ObsReport
}

func (b *blobReceiver) Start(ctx context.Context, host component.Host) error {
	// Encoding extensions are available after the host has started them.
	if err := b.resolveEncodings(host); err != nil {
		return err
	}

	return b.blobEventHandler.run(ctx)
}

func (b *blobReceiver) resolveEncodings(host component.Host) error {
	var err error
	if b.logsDecoder, err = newLogsDecoder(b.logsEncoding, host); err != nil {
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

func (b *blobReceiver) consumeLogs(ctx context.Context, data io.Reader) error {
	if b.nextLogsConsumer == nil {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	reader, err := b.decompress(data)
	if err != nil {
		return fmt.Errorf("failed to decompress logs: %w", err)
	}
	defer reader.Close()

	decoder, err := b.logsDecoder.NewLogsDecoder(reader)
	if err != nil {
		return fmt.Errorf("failed to create logs decoder: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		logs, err := decoder.DecodeLogs()
		if errors.Is(err, io.EOF) {
			// A decoder can finish before reading the gzip trailer.
			if _, readErr := io.Copy(io.Discard, reader); readErr != nil {
				return fmt.Errorf("failed to finish reading logs: %w", readErr)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to decode logs: %w", err)
		}
		count := logs.LogRecordCount()
		if count == 0 {
			continue
		}

		logsContext := b.obsrecv.StartLogsOp(ctx)
		err = b.nextLogsConsumer.ConsumeLogs(logsContext, logs)
		b.obsrecv.EndLogsOp(logsContext, metadata.Type.String(), count, err)
		if err != nil {
			return err
		}
	}
}

func (b *blobReceiver) consumeTraces(ctx context.Context, data io.Reader) error {
	if b.nextTracesConsumer == nil {
		return nil
	}

	reader, err := b.decompress(data)
	if err != nil {
		return fmt.Errorf("failed to decompress traces: %w", err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read traces: %w", err)
	}

	tracesContext := b.obsrecv.StartTracesOp(ctx)

	traces, err := b.tracesUnmarshaler.UnmarshalTraces(payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal traces: %w", err)
	}

	err = b.nextTracesConsumer.ConsumeTraces(tracesContext, traces)

	b.obsrecv.EndTracesOp(tracesContext, metadata.Type.String(), 1, err)

	return err
}

// gzipMagic is the two byte header every gzip stream starts with, see RFC 1952.
var gzipMagic = []byte{0x1f, 0x8b}

// The caller closes the decompressor; the event handler owns the blob reader.
func (b *blobReceiver) decompress(data io.Reader) (io.ReadCloser, error) {
	if b.compression == CompressionNone {
		return io.NopCloser(data), nil
	}

	if b.compression == CompressionAuto {
		buffered := bufio.NewReader(data)
		header, err := buffered.Peek(len(gzipMagic))
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if !bytes.Equal(header, gzipMagic) {
			return io.NopCloser(buffered), nil
		}
		data = buffered
	}

	return gzip.NewReader(data)
}

// Returns a new instance of the log receiver
func newReceiver(set receiver.Settings, eventHandler eventHandler, logsEncoding, tracesEncoding, compression string) (component.Component, error) {
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
		compression:      compression,
		obsrecv:          obsrecv,
	}, nil
}

func newLogsDecoder(name string, host component.Host) (encoding.LogsDecoderFactory, error) {
	switch name {
	case EncodingOTLPJSON:
		return xstreamencoding.NewLogsUnmarshalerDecoderFactory(&plog.JSONUnmarshaler{}), nil
	case EncodingOTLPProto:
		return xstreamencoding.NewLogsUnmarshalerDecoderFactory(&plog.ProtoUnmarshaler{}), nil
	}
	ext, err := encodingExtension(name, host)
	if err != nil {
		return nil, fmt.Errorf("logs.%w", err)
	}
	if decoder, ok := ext.(encoding.LogsDecoderExtension); ok {
		return decoder, nil
	}
	unmarshaler, ok := ext.(plog.Unmarshaler)
	if !ok {
		return nil, fmt.Errorf("logs.encoding extension %q is not a logs decoder or unmarshaler", name)
	}
	return xstreamencoding.NewLogsUnmarshalerDecoderFactory(unmarshaler), nil
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
