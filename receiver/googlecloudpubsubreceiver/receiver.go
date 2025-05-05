// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type pubsubReceiver struct {
	settings           receiver.Settings
	obsrecv            *receiverhelper.ObsReport
	tracesConsumer     consumer.Traces
	metricsConsumer    consumer.Metrics
	logsConsumer       consumer.Logs
	userAgent          string
	config             *Config
	client             internal.SubscriberClient
	tracesUnmarshaler  ptrace.Unmarshaler
	metricsUnmarshaler pmetric.Unmarshaler
	logsUnmarshaler    plog.Unmarshaler
	handler            *internal.StreamHandler
	startOnce          sync.Once
	telemetryBuilder   *metadata.TelemetryBuilder
}

type buildInEncoding int

const (
	unknown         buildInEncoding = iota
	otlpProtoTrace                  = iota
	otlpProtoMetric                 = iota
	otlpProtoLog                    = iota
	rawTextLog                      = iota
	cloudLogging                    = iota
)

type buildInCompression int

const (
	uncompressed buildInCompression = iota
	gZip                            = iota
)

// consumerCount returns the number of attached consumers, useful for detecting errors in pipelines
func (receiver *pubsubReceiver) consumerCount() int {
	count := 0
	if receiver.logsConsumer != nil {
		count++
	}
	if receiver.metricsConsumer != nil {
		count++
	}
	if receiver.tracesConsumer != nil {
		count++
	}
	return count
}

func (receiver *pubsubReceiver) Start(ctx context.Context, host component.Host) error {
	if receiver.tracesConsumer == nil && receiver.metricsConsumer == nil && receiver.logsConsumer == nil {
		return errors.New("cannot start receiver: no consumers were specified")
	}

	var createHandlerFn func(context.Context) error

	if receiver.config.Encoding != "" {
		if receiver.consumerCount() > 1 {
			return errors.New("cannot start receiver: multiple consumers were attached, but encoding was specified")
		}
		encodingID := convertEncoding(receiver.config.Encoding)
		if encodingID == unknown {
			err := receiver.setMarshallerFromExtension(host)
			if err != nil {
				return err
			}
		} else {
			err := receiver.setMarshallerFromEncodingID(encodingID)
			if err != nil {
				return err
			}
		}
		createHandlerFn = receiver.createReceiverHandler
	} else {
		// we will rely on the attributes of the message to determine the signal, so we need all proto unmarshalers
		receiver.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
		receiver.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
		receiver.logsUnmarshaler = &plog.ProtoUnmarshaler{}
		createHandlerFn = receiver.createMultiplexingReceiverHandler
	}

	var startErr error
	receiver.startOnce.Do(func() {
		client, err := newSubscriberClient(ctx, receiver.config, receiver.userAgent)
		if err != nil {
			startErr = fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
			return
		}
		receiver.client = client
		receiver.telemetryBuilder, err = metadata.NewTelemetryBuilder(receiver.settings.TelemetrySettings)
		if err != nil {
			startErr = fmt.Errorf("failed to create telemetry builder: %w", err)
			return
		}

		err = createHandlerFn(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to create ReceiverHandler: %w", err)
			return
		}
	})
	return startErr
}

func (receiver *pubsubReceiver) setMarshallerFromExtension(host component.Host) error {
	extensionID := component.ID{}
	err := extensionID.UnmarshalText([]byte(receiver.config.Encoding))
	if err != nil {
		return errors.New("cannot start receiver: neither a build in encoder, or an extension")
	}
	extensions := host.GetExtensions()
	if extension, ok := extensions[extensionID]; ok {
		if receiver.tracesConsumer != nil {
			receiver.tracesUnmarshaler, ok = extension.(encoding.TracesUnmarshalerExtension)
			if !ok {
				return fmt.Errorf("cannot start receiver: extension %q is not a trace unmarshaler", extensionID)
			}
		}
		if receiver.logsConsumer != nil {
			receiver.logsUnmarshaler, ok = extension.(encoding.LogsUnmarshalerExtension)
			if !ok {
				return fmt.Errorf("cannot start receiver: extension %q is not a logs unmarshaler", extensionID)
			}
		}
		if receiver.metricsConsumer != nil {
			receiver.metricsUnmarshaler, ok = extension.(encoding.MetricsUnmarshalerExtension)
			if !ok {
				return fmt.Errorf("cannot start receiver: extension %q is not a metrics unmarshaler", extensionID)
			}
		}
	} else {
		return fmt.Errorf("cannot start receiver: extension %q not found", extensionID)
	}
	return nil
}

func (receiver *pubsubReceiver) setMarshallerFromEncodingID(encodingID buildInEncoding) error {
	if receiver.tracesConsumer != nil {
		switch encodingID {
		case otlpProtoTrace:
			receiver.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
		default:
			return fmt.Errorf("cannot start receiver: build in encoding %s is not supported for traces", receiver.config.Encoding)
		}
	}
	if receiver.logsConsumer != nil {
		switch encodingID {
		case otlpProtoLog:
			receiver.logsUnmarshaler = &plog.ProtoUnmarshaler{}
		case rawTextLog:
			receiver.logsUnmarshaler = unmarshalLogStrings{}
		case cloudLogging:
			receiver.logsUnmarshaler = unmarshalCloudLoggingLogEntry{}
		default:
			return fmt.Errorf("cannot start receiver: build in encoding %s is not supported for logs", receiver.config.Encoding)
		}
	}
	if receiver.metricsConsumer != nil {
		switch encodingID {
		case otlpProtoMetric:
			receiver.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
		default:
			return fmt.Errorf("cannot start receiver: build in encoding %s is not supported for metrics", receiver.config.Encoding)
		}
	}
	return nil
}

func (receiver *pubsubReceiver) Shutdown(_ context.Context) error {
	if receiver.handler != nil {
		receiver.settings.Logger.Info("Stopping Google Pubsub receiver")
		receiver.handler.CancelNow()
		receiver.settings.Logger.Info("Stopped Google Pubsub receiver")
		receiver.handler = nil
	}
	if receiver.client == nil {
		return nil
	}
	client := receiver.client
	receiver.client = nil
	return client.Close()
}

type unmarshalLogStrings struct{}

func (unmarshalLogStrings) UnmarshalLogs(data []byte) (plog.Logs, error) {
	out := plog.NewLogs()
	logs := out.ResourceLogs()
	rls := logs.AppendEmpty()

	ills := rls.ScopeLogs().AppendEmpty()
	lr := ills.LogRecords().AppendEmpty()

	lr.Body().SetStr(string(data))
	return out, nil
}

func (receiver *pubsubReceiver) handleLogStrings(ctx context.Context, payload []byte) error {
	if receiver.logsConsumer == nil {
		return nil
	}
	unmarshall := unmarshalLogStrings{}
	out, err := unmarshall.UnmarshalLogs(payload)
	if err != nil {
		return err
	}
	return receiver.logsConsumer.ConsumeLogs(ctx, out)
}

type unmarshalCloudLoggingLogEntry struct{}

func (unmarshalCloudLoggingLogEntry) UnmarshalLogs(data []byte) (plog.Logs, error) {
	resource, lr, err := internal.TranslateLogEntry(data)
	out := plog.NewLogs()

	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	if err != nil {
		return out, err
	}

	logs := out.ResourceLogs()
	rls := logs.AppendEmpty()
	resource.CopyTo(rls.Resource())

	ills := rls.ScopeLogs().AppendEmpty()
	lr.CopyTo(ills.LogRecords().AppendEmpty())

	return out, nil
}

func decompress(payload []byte, compression buildInCompression) ([]byte, error) {
	if compression == gZip {
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	}
	return payload, nil
}

func (receiver *pubsubReceiver) handleTrace(ctx context.Context, payload []byte, compression buildInCompression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.tracesUnmarshaler.UnmarshalTraces(payload)
	if err != nil {
		receiver.increaseEncodingErrorMetric(ctx, "traces")
		if receiver.config.IgnoreEncodingError {
			return nil
		}
		return err
	}
	count := otlpData.SpanCount()
	ctx = receiver.obsrecv.StartTracesOp(ctx)
	err = receiver.tracesConsumer.ConsumeTraces(ctx, otlpData)
	receiver.obsrecv.EndTracesOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) handleMetric(ctx context.Context, payload []byte, compression buildInCompression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.metricsUnmarshaler.UnmarshalMetrics(payload)
	if err != nil {
		receiver.increaseEncodingErrorMetric(ctx, "metrics")
		if receiver.config.IgnoreEncodingError {
			return nil
		}
		return err
	}
	count := otlpData.MetricCount()
	ctx = receiver.obsrecv.StartMetricsOp(ctx)
	err = receiver.metricsConsumer.ConsumeMetrics(ctx, otlpData)
	receiver.obsrecv.EndMetricsOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) handleLog(ctx context.Context, payload []byte, compression buildInCompression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.logsUnmarshaler.UnmarshalLogs(payload)
	if err != nil {
		receiver.increaseEncodingErrorMetric(ctx, "logs")
		if receiver.config.IgnoreEncodingError {
			return nil
		}
		return err
	}
	count := otlpData.LogRecordCount()
	ctx = receiver.obsrecv.StartLogsOp(ctx)
	err = receiver.logsConsumer.ConsumeLogs(ctx, otlpData)
	receiver.obsrecv.EndLogsOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) increaseEncodingErrorMetric(ctx context.Context, signal string) {
	receiver.telemetryBuilder.ReceiverGooglecloudpubsubEncodingError.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("otelcol.component.kind", "receiver"),
			attribute.String("otelcol.component.id", receiver.settings.ID.String()),
			attribute.String("otelcol.signal", signal),
		))
}

func (receiver *pubsubReceiver) detectEncoding(attributes map[string]string) (otlpEncoding buildInEncoding, otlpCompression buildInCompression) {
	otlpEncoding = unknown
	otlpCompression = uncompressed

	ceType := attributes["ce-type"]
	ceContentType := attributes["content-type"]
	if strings.HasSuffix(ceContentType, "application/protobuf") {
		switch ceType {
		case "org.opentelemetry.otlp.traces.v1":
			otlpEncoding = otlpProtoTrace
		case "org.opentelemetry.otlp.metrics.v1":
			otlpEncoding = otlpProtoMetric
		case "org.opentelemetry.otlp.logs.v1":
			otlpEncoding = otlpProtoLog
		}
	} else if strings.HasSuffix(ceContentType, "text/plain") {
		otlpEncoding = rawTextLog
	}

	if otlpEncoding == unknown && receiver.config.Encoding != "" {
		otlpEncoding = convertEncoding(receiver.config.Encoding)
	}

	ceContentEncoding := attributes["content-encoding"]
	if ceContentEncoding == "gzip" {
		otlpCompression = gZip
	}

	if otlpCompression == uncompressed && receiver.config.Compression != "" {
		if receiver.config.Compression == "gzip" {
			otlpCompression = gZip
		}
	}
	return
}

func convertEncoding(encodingConfig string) (encoding buildInEncoding) {
	switch encodingConfig {
	case "otlp_proto_trace":
		return otlpProtoTrace
	case "otlp_proto_metric":
		return otlpProtoMetric
	case "otlp_proto_log":
		return otlpProtoLog
	case "cloud_logging":
		return cloudLogging
	case "raw_text":
		return rawTextLog
	}
	return unknown
}

func (receiver *pubsubReceiver) createMultiplexingReceiverHandler(ctx context.Context) error {
	var err error
	receiver.handler, err = internal.NewHandler(
		ctx,
		receiver.settings,
		receiver.telemetryBuilder,
		receiver.client,
		receiver.config.ClientID,
		receiver.config.Subscription,
		func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			payload := message.Message.Data
			encoding, compression := receiver.detectEncoding(message.Message.Attributes)

			switch encoding {
			case otlpProtoTrace:
				if receiver.tracesConsumer != nil {
					return receiver.handleTrace(ctx, payload, compression)
				}
			case otlpProtoMetric:
				if receiver.metricsConsumer != nil {
					return receiver.handleMetric(ctx, payload, compression)
				}
			case otlpProtoLog:
				if receiver.logsConsumer != nil {
					return receiver.handleLog(ctx, payload, compression)
				}
			case rawTextLog:
				if receiver.logsConsumer != nil {
					return receiver.handleLogStrings(ctx, payload)
				}
			default:
				return errors.New("unknown encoding")
			}
			return nil
		})
	if err != nil {
		return err
	}
	receiver.handler.RecoverableStream(ctx)
	return nil
}

func (receiver *pubsubReceiver) createReceiverHandler(ctx context.Context) error {
	var err error
	var handlerFn func(context.Context, *pubsubpb.ReceivedMessage) error
	compression := uncompressed
	if receiver.tracesConsumer != nil {
		handlerFn = func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			payload := message.Message.Data
			return receiver.handleTrace(ctx, payload, compression)
		}
	}
	if receiver.logsConsumer != nil {
		handlerFn = func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			payload := message.Message.Data
			return receiver.handleLog(ctx, payload, compression)
		}
	}
	if receiver.metricsConsumer != nil {
		handlerFn = func(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
			payload := message.Message.Data
			return receiver.handleMetric(ctx, payload, compression)
		}
	}

	receiver.handler, err = internal.NewHandler(
		ctx,
		receiver.settings,
		receiver.telemetryBuilder,
		receiver.client,
		receiver.config.ClientID,
		receiver.config.Subscription,
		handlerFn)
	if err != nil {
		return err
	}
	receiver.handler.RecoverableStream(ctx)
	return nil
}
