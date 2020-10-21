// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
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
	tracesUnmarshaler  pdata.TracesUnmarshaler
	metricsUnmarshaler pdata.MetricsUnmarshaler
	logsUnmarshaler    pdata.LogsUnmarshaler
	handler            *internal.StreamHandler
	startOnce          sync.Once
}

type Encoding int

const (
	Unknown         Encoding = iota
	OtlpProtoTrace           = iota
	OtlpProtoMetric          = iota
	OtlpProtoLog             = iota
	RawTextLog               = iota
)

type Compression int

const (
	Uncompressed Compression = iota
	GZip                     = iota
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
	receiver.tracesUnmarshaler = otlp.NewProtobufTracesUnmarshaler()
	receiver.metricsUnmarshaler = otlp.NewProtobufMetricsUnmarshaler()
	receiver.logsUnmarshaler = otlp.NewProtobufLogsUnmarshaler()
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

	out := pdata.NewLogs()
	logs := out.ResourceLogs()
	rls := logs.AppendEmpty()

	ills := rls.InstrumentationLibraryLogs().AppendEmpty()
	lr := ills.Logs().AppendEmpty()

	lr.Body().SetStringVal(data)
	lr.SetTimestamp(pdata.NewTimestampFromTime(timestamp.AsTime()))
	return receiver.logsConsumer.ConsumeLogs(ctx, out)
}

func decompress(payload []byte, compression Compression) ([]byte, error) {
	switch compression {
	case GZip:
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(reader)
	}
	return payload, nil
}

func (receiver *pubsubReceiver) handleTrace(ctx context.Context, payload []byte, compression Compression) error {
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

func (receiver *pubsubReceiver) handleMetric(ctx context.Context, payload []byte, compression Compression) error {
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

func (receiver *pubsubReceiver) handleLog(ctx context.Context, payload []byte, compression Compression) error {
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

func (receiver *pubsubReceiver) detectEncoding(attributes map[string]string) (Encoding, Compression) {
	otlpEncoding := Unknown
	otlpCompression := Uncompressed

	ceType := attributes["ce-type"]
	ceContentType := attributes["content-type"]
	if strings.HasSuffix(ceContentType, "application/protobuf") {
		switch ceType {
		case "org.opentelemetry.otlp.traces.v1":
			otlpEncoding = OtlpProtoTrace
		case "org.opentelemetry.otlp.metrics.v1":
			otlpEncoding = OtlpProtoMetric
		case "org.opentelemetry.otlp.logs.v1":
			otlpEncoding = OtlpProtoLog
		}
	} else if strings.HasSuffix(ceContentType, "text/plain") {
		otlpEncoding = RawTextLog
	}

	if otlpEncoding == Unknown && receiver.config.Encoding != "" {
		switch receiver.config.Encoding {
		case "otlp_proto_trace":
			otlpEncoding = OtlpProtoTrace
		case "otlp_proto_metric":
			otlpEncoding = OtlpProtoMetric
		case "otlp_proto_log":
			otlpEncoding = OtlpProtoLog
		case "raw_text":
			otlpEncoding = RawTextLog
		}
	}

	ceContentEncoding := attributes["content-encoding"]
	switch ceContentEncoding {
	case "gzip":
		otlpCompression = GZip
	}

	if otlpCompression == Uncompressed && receiver.config.Compression != "" {
		switch receiver.config.Compression {
		case "gzip":
			otlpCompression = GZip
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
			case OtlpProtoTrace:
				if receiver.tracesConsumer != nil {
					return receiver.handleTrace(ctx, payload, compression)
				}
			case OtlpProtoMetric:
				if receiver.metricsConsumer != nil {
					return receiver.handleMetric(ctx, payload, compression)
				}
			case OtlpProtoLog:
				if receiver.logsConsumer != nil {
					return receiver.handleLog(ctx, payload, compression)
				}
			case RawTextLog:
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
