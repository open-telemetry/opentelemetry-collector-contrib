// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

type encodingExtension struct {
	extension component.Component
	suffix    string
}

type encodingExtensions []encodingExtension

type receiverProcessor interface {
	processReceivedData(ctx context.Context, receiver *awss3Receiver, key string, data io.Reader) error
}

type awss3Receiver struct {
	reader           s3Reader
	logger           *zap.Logger
	cancel           context.CancelFunc
	obsrecv          *receiverhelper.ObsReport
	encodingsConfig  []Encoding
	telemetryType    string
	dataProcessor    receiverProcessor
	extensions       encodingExtensions
	notifier         statusNotifier
	decompression    configcompression.Type
	maxUnmarshalSize int64
}

func newAWSS3Receiver(ctx context.Context, cfg *Config, telemetryType string, settings receiver.Settings, processor receiverProcessor) (*awss3Receiver, error) {
	notifier := newNotifier(cfg, settings.Logger)
	var reader s3Reader
	var err error

	// Create the appropriate reader based on configuration
	switch {
	case cfg.StartTime != "" && cfg.EndTime != "":
		reader, err = newS3TimeBasedReader(ctx, notifier, settings.Logger, cfg)
		if err != nil {
			return nil, err
		}
	case cfg.SQS != nil:
		reader, err = newS3SQSReader(ctx, settings.Logger, cfg)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("invalid configuration: either time-based (StartTime/EndTime) or SQS-based configuration must be provided")
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "s3",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	return &awss3Receiver{
		reader:           reader,
		telemetryType:    telemetryType,
		logger:           settings.Logger,
		cancel:           nil,
		obsrecv:          obsrecv,
		dataProcessor:    processor,
		encodingsConfig:  cfg.Encodings,
		notifier:         notifier,
		decompression:    cfg.S3Downloader.Decompression,
		maxUnmarshalSize: cfg.S3Downloader.MaxUnmarshalSize,
	}, nil
}

func (r *awss3Receiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if r.notifier != nil {
		err = r.notifier.Start(ctx, host)
		if err != nil {
			return err
		}
	}
	r.extensions, err = newEncodingExtensions(r.encodingsConfig, host)
	if err != nil {
		return err
	}

	var cancelCtx context.Context
	cancelCtx, r.cancel = context.WithCancel(context.Background())
	go func() {
		_ = r.reader.readAll(cancelCtx, r.telemetryType, r.receive)
	}()
	return nil
}

func (r *awss3Receiver) Shutdown(ctx context.Context) error {
	if r.notifier != nil {
		if err := r.notifier.Shutdown(ctx); err != nil {
			return err
		}
	}
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

func (r *awss3Receiver) receive(ctx context.Context, key string, data io.Reader) (err error) {
	if data == nil {
		return nil
	}
	err = ctx.Err()
	if err != nil {
		return err
	}
	algorithm := r.decompression
	switch {
	case strings.HasSuffix(key, ".gz"):
		key = strings.TrimSuffix(key, ".gz")
		algorithm = configcompression.TypeGzip
	case strings.HasSuffix(key, ".zst"):
		key = strings.TrimSuffix(key, ".zst")
		algorithm = configcompression.TypeZstd
	}
	if algorithm.IsCompressed() {
		var decompressor io.ReadCloser
		decompressor, err = newDecompressor(algorithm, data)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, decompressor.Close())
		}()
		data = decompressor
	}
	err = r.dataProcessor.processReceivedData(ctx, r, key, data)
	if err != nil {
		return err
	}
	// A decoder may stop before the end of the object. Read through EOF to
	// validate the download and compression checksum before acknowledging it.
	err = copyReader(ctx, io.Discard, data)
	if err != nil {
		return err
	}
	return ctx.Err()
}

func newDecompressor(algorithm configcompression.Type, data io.Reader) (io.ReadCloser, error) {
	switch algorithm {
	case configcompression.TypeGzip:
		return gzip.NewReader(data)
	case configcompression.TypeZstd:
		decoder, err := zstd.NewReader(data)
		if err != nil {
			return nil, err
		}
		return decoder.IOReadCloser(), nil
	default:
		return nil, fmt.Errorf("unsupported decompression type %q", algorithm)
	}
}

type traceReceiver struct {
	consumer consumer.Traces
}

func newAWSS3TraceReceiver(ctx context.Context, cfg *Config, traces consumer.Traces, settings receiver.Settings) (*awss3Receiver, error) {
	return newAWSS3Receiver(ctx, cfg, "traces", settings, &traceReceiver{consumer: traces})
}

func (r *traceReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data io.Reader) error {
	var unmarshaler ptrace.Unmarshaler
	var format string

	if extension, f := rcvr.extensions.findExtension(key); extension != nil {
		unmarshaler, _ = extension.(ptrace.Unmarshaler)
		format = f
	}

	if unmarshaler == nil {
		if strings.HasSuffix(key, ".json") {
			unmarshaler = &ptrace.JSONUnmarshaler{}
			format = "otlp_json"
		}
		if strings.HasSuffix(key, ".binpb") {
			unmarshaler = &ptrace.ProtoUnmarshaler{}
			format = "otlp_proto"
		}
	}
	if unmarshaler == nil {
		rcvr.logger.Warn("Unsupported file format", zap.String("key", key))
		return nil
	}
	rcvr.logger.Debug("Processing trace file", zap.String("key", key), zap.String("format", format))
	buf, err := readAll(ctx, data, rcvr.maxUnmarshalSize)
	if err != nil {
		return err
	}
	traces, err := unmarshaler.UnmarshalTraces(buf)
	if err != nil {
		return err
	}
	obsCtx := rcvr.obsrecv.StartTracesOp(ctx)
	err = r.consumer.ConsumeTraces(ctx, traces)
	rcvr.obsrecv.EndTracesOp(obsCtx, format, traces.SpanCount(), err)
	return err
}

type metricsReceiver struct {
	consumer consumer.Metrics
}

func newAWSS3MetricsReceiver(ctx context.Context, cfg *Config, metrics consumer.Metrics, settings receiver.Settings) (*awss3Receiver, error) {
	return newAWSS3Receiver(ctx, cfg, "metrics", settings, &metricsReceiver{consumer: metrics})
}

func (r *metricsReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data io.Reader) error {
	var unmarshaler pmetric.Unmarshaler
	var format string

	if extension, f := rcvr.extensions.findExtension(key); extension != nil {
		if factory, ok := extension.(encoding.MetricsDecoderFactory); ok {
			decoder, err := factory.NewMetricsDecoder(data)
			if err != nil {
				return err
			}
			return r.consumeStream(ctx, rcvr, f, decoder)
		}
		unmarshaler, _ = extension.(pmetric.Unmarshaler)
		format = f
	}

	if unmarshaler == nil {
		if strings.HasSuffix(key, ".json") {
			unmarshaler = &pmetric.JSONUnmarshaler{}
			format = "otlp_json"
		}
		if strings.HasSuffix(key, ".binpb") {
			unmarshaler = &pmetric.ProtoUnmarshaler{}
			format = "otlp_proto"
		}
	}
	if unmarshaler == nil {
		rcvr.logger.Warn("Unsupported file format", zap.String("key", key))
		return nil
	}
	rcvr.logger.Debug("Processing metric file", zap.String("key", key), zap.String("format", format))
	buf, err := readAll(ctx, data, rcvr.maxUnmarshalSize)
	if err != nil {
		return err
	}
	metrics, err := unmarshaler.UnmarshalMetrics(buf)
	if err != nil {
		return err
	}
	obsCtx := rcvr.obsrecv.StartMetricsOp(ctx)
	err = r.consumer.ConsumeMetrics(ctx, metrics)
	rcvr.obsrecv.EndMetricsOp(obsCtx, format, metrics.MetricCount(), err)
	return err
}

type logsReceiver struct {
	consumer consumer.Logs
}

func newAWSS3LogsReceiver(ctx context.Context, cfg *Config, logs consumer.Logs, settings receiver.Settings) (*awss3Receiver, error) {
	return newAWSS3Receiver(ctx, cfg, "logs", settings, &logsReceiver{consumer: logs})
}

func (r *logsReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data io.Reader) error {
	var unmarshaler plog.Unmarshaler
	var format string

	if extension, f := rcvr.extensions.findExtension(key); extension != nil {
		if factory, ok := extension.(encoding.LogsDecoderFactory); ok {
			decoder, err := factory.NewLogsDecoder(data)
			if err != nil {
				return err
			}
			return r.consumeStream(ctx, rcvr, f, decoder)
		}
		unmarshaler, _ = extension.(plog.Unmarshaler)
		format = f
	}

	if unmarshaler == nil {
		if strings.HasSuffix(key, ".json") {
			unmarshaler = &plog.JSONUnmarshaler{}
			format = "otlp_json"
		}
		if strings.HasSuffix(key, ".binpb") {
			unmarshaler = &plog.ProtoUnmarshaler{}
			format = "otlp_proto"
		}
	}
	if unmarshaler == nil {
		rcvr.logger.Warn("Unsupported file format", zap.String("key", key))
		return nil
	}
	rcvr.logger.Debug("Processing log file", zap.String("key", key), zap.String("format", format))
	buf, err := readAll(ctx, data, rcvr.maxUnmarshalSize)
	if err != nil {
		return err
	}
	logs, err := unmarshaler.UnmarshalLogs(buf)
	if err != nil {
		return err
	}
	obsCtx := rcvr.obsrecv.StartLogsOp(ctx)
	err = r.consumer.ConsumeLogs(ctx, logs)
	rcvr.obsrecv.EndLogsOp(obsCtx, format, logs.LogRecordCount(), err)
	return err
}

func (r *logsReceiver) consumeStream(ctx context.Context, rcvr *awss3Receiver, format string, decoder encoding.LogsDecoder) error {
	abort := ctx.Done()
	for {
		select {
		case <-abort:
			return ctx.Err()
		default:
		}
		logs, err := decoder.DecodeLogs()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		obsCtx := rcvr.obsrecv.StartLogsOp(ctx)
		err = r.consumer.ConsumeLogs(ctx, logs)
		rcvr.obsrecv.EndLogsOp(obsCtx, format, logs.LogRecordCount(), err)
		if err != nil {
			return err
		}
	}
}

func (r *metricsReceiver) consumeStream(ctx context.Context, rcvr *awss3Receiver, format string, decoder encoding.MetricsDecoder) error {
	abort := ctx.Done()
	for {
		select {
		case <-abort:
			return ctx.Err()
		default:
		}
		metrics, err := decoder.DecodeMetrics()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		obsCtx := rcvr.obsrecv.StartMetricsOp(ctx)
		err = r.consumer.ConsumeMetrics(ctx, metrics)
		rcvr.obsrecv.EndMetricsOp(obsCtx, format, metrics.MetricCount(), err)
		if err != nil {
			return err
		}
	}
}

func newEncodingExtensions(encodingsConfig []Encoding, host component.Host) (encodingExtensions, error) {
	encodings := make(encodingExtensions, 0)
	extensions := host.GetExtensions()
	for _, configItem := range encodingsConfig {
		e, ok := extensions[configItem.Extension]
		if !ok {
			return nil, fmt.Errorf("extension %q not found", configItem.Extension)
		}
		encodings = append(encodings, encodingExtension{extension: e, suffix: configItem.Suffix})
	}
	return encodings, nil
}

func (encodings encodingExtensions) findExtension(key string) (component.Component, string) {
	for _, e := range encodings {
		if strings.HasSuffix(key, e.suffix) {
			return e.extension, e.suffix
		}
	}
	return nil, ""
}
