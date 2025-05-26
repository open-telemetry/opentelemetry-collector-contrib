// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

type encodingExtension struct {
	extension component.Component
	suffix    string
}

type encodingExtensions []encodingExtension

type receiverProcessor interface {
	processReceivedData(ctx context.Context, receiver *awss3Receiver, key string, data []byte) error
}

type awss3Receiver struct {
	reader          s3Reader
	logger          *zap.Logger
	cancel          context.CancelFunc
	obsrecv         *receiverhelper.ObsReport
	encodingsConfig []Encoding
	telemetryType   string
	dataProcessor   receiverProcessor
	extensions      encodingExtensions
	notifier        statusNotifier
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
		reader:          reader,
		telemetryType:   telemetryType,
		logger:          settings.Logger,
		cancel:          nil,
		obsrecv:         obsrecv,
		dataProcessor:   processor,
		encodingsConfig: cfg.Encodings,
		notifier:        notifier,
	}, nil
}

func (r *awss3Receiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if r.notifier != nil {
		if err = r.notifier.Start(ctx, host); err != nil {
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
		_ = r.reader.readAll(cancelCtx, r.telemetryType, r.receiveBytes)
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

func (r *awss3Receiver) receiveBytes(ctx context.Context, key string, data []byte) error {
	if data == nil {
		return nil
	}
	if strings.HasSuffix(key, ".gz") {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		key = strings.TrimSuffix(key, ".gz")
		data, err = io.ReadAll(reader)
		if err != nil {
			return err
		}
	}
	return r.dataProcessor.processReceivedData(ctx, r, key, data)
}

type traceReceiver struct {
	consumer consumer.Traces
}

func newAWSS3TraceReceiver(ctx context.Context, cfg *Config, traces consumer.Traces, settings receiver.Settings) (*awss3Receiver, error) {
	return newAWSS3Receiver(ctx, cfg, "traces", settings, &traceReceiver{consumer: traces})
}

func (r *traceReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data []byte) error {
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
	traces, err := unmarshaler.UnmarshalTraces(data)
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

func (r *metricsReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data []byte) error {
	var unmarshaler pmetric.Unmarshaler
	var format string

	if extension, f := rcvr.extensions.findExtension(key); extension != nil {
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
	metrics, err := unmarshaler.UnmarshalMetrics(data)
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

func (r *logsReceiver) processReceivedData(ctx context.Context, rcvr *awss3Receiver, key string, data []byte) error {
	var unmarshaler plog.Unmarshaler
	var format string

	if extension, f := rcvr.extensions.findExtension(key); extension != nil {
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
	logs, err := unmarshaler.UnmarshalLogs(data)
	if err != nil {
		return err
	}
	obsCtx := rcvr.obsrecv.StartLogsOp(ctx)
	err = r.consumer.ConsumeLogs(ctx, logs)
	rcvr.obsrecv.EndLogsOp(obsCtx, format, logs.LogRecordCount(), err)
	return err
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
