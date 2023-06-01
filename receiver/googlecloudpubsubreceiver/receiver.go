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

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
)

// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type pubsubReceiver struct {
	logger             *zap.Logger
	obsrecv            *obsreport.Receiver
	tracesConsumer     consumer.Traces
	metricsConsumer    consumer.Metrics
	logsConsumer       consumer.Logs
	userAgent          string
	config             *Config
	client             *pubsub.SubscriberClient
	tracesUnmarshaler  ptrace.Unmarshaler
	metricsUnmarshaler pmetric.Unmarshaler
	logsUnmarshaler    plog.Unmarshaler
	handler            *internal.StreamHandler
	startOnce          sync.Once
}

type encoding int

const (
	unknown         encoding = iota
	otlpProtoTrace           = iota
	otlpProtoMetric          = iota
	otlpProtoLog             = iota
	rawTextLog               = iota
)

type compression int

const (
	uncompressed compression = iota
	gZip                     = iota
)

func (receiver *pubsubReceiver) generateClientOptions() (copts []option.ClientOption) {
	if receiver.userAgent != "" {
		copts = append(copts, option.WithUserAgent(receiver.userAgent))
	}
	if receiver.config.Endpoint != "" {
		if receiver.config.Insecure {
			var dialOpts []grpc.DialOption
			if receiver.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(receiver.userAgent))
			}
			conn, _ := grpc.Dial(receiver.config.Endpoint, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(receiver.config.Endpoint))
		}
	}
	return copts
}

func (receiver *pubsubReceiver) Start(ctx context.Context, _ component.Host) error {
	if receiver.tracesConsumer == nil && receiver.metricsConsumer == nil && receiver.logsConsumer == nil {
		return errors.New("cannot start receiver: no consumers were specified")
	}

	var startErr error
	receiver.startOnce.Do(func() {
		copts := receiver.generateClientOptions()
		client, err := pubsub.NewSubscriberClient(ctx, copts...)
		if err != nil {
			startErr = fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
			return
		}
		receiver.client = client

		err = receiver.createReceiverHandler(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to create ReceiverHandler: %w", err)
			return
		}
	})
	receiver.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
	receiver.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
	receiver.logsUnmarshaler = &plog.ProtoUnmarshaler{}
	return startErr
}

func (receiver *pubsubReceiver) Shutdown(_ context.Context) error {
	receiver.logger.Info("Stopping Google Pubsub receiver")
	receiver.handler.CancelNow()
	receiver.logger.Info("Stopped Google Pubsub receiver")
	return nil
}

func (receiver *pubsubReceiver) handleLogStrings(ctx context.Context, message *pubsubpb.ReceivedMessage) error {
	if receiver.logsConsumer == nil {
		return nil
	}
	data := string(message.Message.Data)
	timestamp := message.GetMessage().PublishTime

	out := plog.NewLogs()
	logs := out.ResourceLogs()
	rls := logs.AppendEmpty()

	ills := rls.ScopeLogs().AppendEmpty()
	lr := ills.LogRecords().AppendEmpty()

	lr.Body().SetStr(data)
	lr.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.AsTime()))
	return receiver.logsConsumer.ConsumeLogs(ctx, out)
}

func decompress(payload []byte, compression compression) ([]byte, error) {
	if compression == gZip {
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	}
	return payload, nil
}

func (receiver *pubsubReceiver) handleTrace(ctx context.Context, payload []byte, compression compression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.tracesUnmarshaler.UnmarshalTraces(payload)
	count := otlpData.SpanCount()
	if err != nil {
		return err
	}
	ctx = receiver.obsrecv.StartTracesOp(ctx)
	err = receiver.tracesConsumer.ConsumeTraces(ctx, otlpData)
	receiver.obsrecv.EndTracesOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) handleMetric(ctx context.Context, payload []byte, compression compression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.metricsUnmarshaler.UnmarshalMetrics(payload)
	count := otlpData.MetricCount()
	if err != nil {
		return err
	}
	ctx = receiver.obsrecv.StartMetricsOp(ctx)
	err = receiver.metricsConsumer.ConsumeMetrics(ctx, otlpData)
	receiver.obsrecv.EndMetricsOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) handleLog(ctx context.Context, payload []byte, compression compression) error {
	payload, err := decompress(payload, compression)
	if err != nil {
		return err
	}
	otlpData, err := receiver.logsUnmarshaler.UnmarshalLogs(payload)
	count := otlpData.LogRecordCount()
	if err != nil {
		return err
	}
	ctx = receiver.obsrecv.StartLogsOp(ctx)
	err = receiver.logsConsumer.ConsumeLogs(ctx, otlpData)
	receiver.obsrecv.EndLogsOp(ctx, reportFormatProtobuf, count, err)
	return nil
}

func (receiver *pubsubReceiver) detectEncoding(attributes map[string]string) (encoding, compression) {
	otlpEncoding := unknown
	otlpCompression := uncompressed

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
		switch receiver.config.Encoding {
		case "otlp_proto_trace":
			otlpEncoding = otlpProtoTrace
		case "otlp_proto_metric":
			otlpEncoding = otlpProtoMetric
		case "otlp_proto_log":
			otlpEncoding = otlpProtoLog
		case "raw_text":
			otlpEncoding = rawTextLog
		}
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
	return otlpEncoding, otlpCompression
}

func (receiver *pubsubReceiver) createReceiverHandler(ctx context.Context) error {
	var err error
	receiver.handler, err = internal.NewHandler(
		ctx,
		receiver.logger,
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
				return receiver.handleLogStrings(ctx, message)
			}
			return errors.New("unknown encoding")
		})
	if err != nil {
		return err
	}
	receiver.handler.RecoverableStream(ctx)
	return nil
}
