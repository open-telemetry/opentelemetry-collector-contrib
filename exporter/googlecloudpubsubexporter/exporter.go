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

package googlecloudpubsubexporter

import (
	"context"
	"fmt"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
)

const name = "googlecloudpubsub"

type pubsubExporter struct {
	instanceName     string
	logger           *zap.Logger
	topicName        string
	client           *pubsub.PublisherClient
	cancel           context.CancelFunc
	userAgent        string
	ceSource         string
	config           *Config
	tracesMarshaler  pdata.TracesMarshaler
	metricsMarshaler pdata.MetricsMarshaler
	logsMarshaler    pdata.LogsMarshaler
}

func (*pubsubExporter) Name() string {
	return name
}

type Encoding int

const (
	OtlpProtoTrace  = iota
	OtlpProtoMetric = iota
	OtlpProtoLog    = iota
)

func (ex *pubsubExporter) start(ctx context.Context, _ component.Host) error {
	ctx, ex.cancel = context.WithCancel(ctx)

	if ex.client == nil {
		copts := ex.generateClientOptions()
		client, err := pubsub.NewPublisherClient(ctx, copts...)
		if err != nil {
			return fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
		}

		ex.client = client
	}
	ex.tracesMarshaler = otlp.NewProtobufTracesMarshaler()
	ex.metricsMarshaler = otlp.NewProtobufMetricsMarshaler()
	ex.logsMarshaler = otlp.NewProtobufLogsMarshaler()
	return nil
}

func (ex *pubsubExporter) shutdown(context.Context) error {
	if ex.client != nil {
		ex.client.Close()
		ex.client = nil
	}
	return nil
}

func (ex *pubsubExporter) generateClientOptions() []option.ClientOption {
	var copts []option.ClientOption
	if ex.userAgent != "" {
		copts = append(copts, option.WithUserAgent(ex.userAgent))
	}
	if ex.config.Endpoint != "" {
		if ex.config.Insecure {
			var dialOpts []grpc.DialOption
			if ex.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(ex.userAgent))
			}
			conn, _ := grpc.Dial(ex.config.Endpoint, append(dialOpts, grpc.WithInsecure())...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(ex.config.Endpoint))
		}
	}
	return copts
}

func (ex *pubsubExporter) publishMessage(ctx context.Context, encoding Encoding, data []byte) error {
	id, _ := uuid.NewRandom()
	attributes := map[string]string{
		"ce-specversion": "1.0",
		"ce-id":          id.String(),
		"ce-source":      ex.ceSource,
	}
	switch encoding {
	case OtlpProtoTrace:
		attributes["ce-type"] = "org.opentelemetry.otlp.traces.v1"
		attributes["content-type"] = "application/protobuf"
	case OtlpProtoMetric:
		attributes["ce-type"] = "org.opentelemetry.otlp.metrics.v1"
		attributes["content-type"] = "application/protobuf"
	case OtlpProtoLog:
		attributes["ce-type"] = "org.opentelemetry.otlp.logs.v1"
		attributes["content-type"] = "application/protobuf"
	}
	_, err := ex.client.Publish(ctx, &pubsubpb.PublishRequest{
		Topic: ex.topicName,
		Messages: []*pubsubpb.PubsubMessage{
			{
				Attributes: attributes,
				Data:       data,
			},
		},
	})
	return err
}

func (ex *pubsubExporter) consumeTraces(ctx context.Context, traces pdata.Traces) error {
	bytes, _ := ex.tracesMarshaler.MarshalTraces(traces)
	return ex.publishMessage(ctx, OtlpProtoTrace, bytes)
}

func (ex *pubsubExporter) consumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	bytes, _ := ex.metricsMarshaler.MarshalMetrics(metrics)
	return ex.publishMessage(ctx, OtlpProtoMetric, bytes)
}

func (ex *pubsubExporter) consumeLogs(ctx context.Context, logs pdata.Logs) error {
	bytes, _ := ex.logsMarshaler.MarshalLogs(logs)
	return ex.publishMessage(ctx, OtlpProtoLog, bytes)
}
