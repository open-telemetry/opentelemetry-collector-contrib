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

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"time"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const name = "googlecloudpubsub"

type pubsubExporter struct {
	instanceName         string
	logger               *zap.Logger
	topicName            string
	client               *pubsub.PublisherClient
	cancel               context.CancelFunc
	userAgent            string
	ceSource             string
	ceCompression        Compression
	config               *Config
	tracesMarshaler      pdata.TracesMarshaler
	tracesWatermarkFunc  TracesWatermarkFunc
	metricsMarshaler     pdata.MetricsMarshaler
	metricsWatermarkFunc MetricsWatermarkFunc
	logsMarshaler        pdata.LogsMarshaler
	logsWatermarkFunc    LogsWatermarkFunc
}

func (*pubsubExporter) Name() string {
	return name
}

type Encoding int

const (
	OtlpProtoTrace  Encoding = iota
	OtlpProtoMetric          = iota
	OtlpProtoLog             = iota
)

type Compression int

const (
	Uncompressed Compression = iota
	GZip                     = iota
)

type WatermarkBehavior int

const (
	Current  WatermarkBehavior = iota
	Earliest                   = iota
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
	ex.tracesWatermarkFunc = currentTracesWatermark
	ex.metricsMarshaler = otlp.NewProtobufMetricsMarshaler()
	ex.metricsWatermarkFunc = currentMetricsWatermark
	ex.logsMarshaler = otlp.NewProtobufLogsMarshaler()
	ex.logsWatermarkFunc = currentLogsWatermark
	return nil
}

func (ex *pubsubExporter) shutdown(context.Context) error {
	if ex.client != nil {
		ex.client.Close()
		ex.client = nil
	}
	return nil
}

func (ex *pubsubExporter) generateClientOptions() (copts []option.ClientOption) {
	if ex.userAgent != "" {
		copts = append(copts, option.WithUserAgent(ex.userAgent))
	}
	if ex.config.Endpoint != "" {
		if ex.config.Insecure {
			var dialOpts []grpc.DialOption
			if ex.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(ex.userAgent))
			}
			conn, _ := grpc.Dial(ex.config.Endpoint, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(ex.config.Endpoint))
		}
	}
	return copts
}

func (ex *pubsubExporter) publishMessage(ctx context.Context, encoding Encoding, data []byte, watermark time.Time) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	ceTime, err := watermark.MarshalText()
	if err != nil {
		return err
	}

	attributes := map[string]string{
		"ce-specversion": "1.0",
		"ce-id":          id.String(),
		"ce-source":      ex.ceSource,
		"ce-time":        string(ceTime),
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
	switch ex.ceCompression {
	case GZip:
		attributes["content-encoding"] = "gzip"
		data, err = ex.compress(data)
		if err != nil {
			return err
		}
	}
	_, err = ex.client.Publish(ctx, &pubsubpb.PublishRequest{
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

func (ex *pubsubExporter) compress(payload []byte) ([]byte, error) {
	switch ex.ceCompression {
	case GZip:
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		_, err := writer.Write(payload)
		if err != nil {
			return nil, err
		}
		err = writer.Close()
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return payload, nil
}

func (ex *pubsubExporter) consumeTraces(ctx context.Context, traces pdata.Traces) error {
	buffer, err := ex.tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, OtlpProtoTrace, buffer, ex.tracesWatermarkFunc(traces, time.Now(), time.Hour*1).UTC())
}

func (ex *pubsubExporter) consumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	buffer, err := ex.metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, OtlpProtoMetric, buffer, ex.metricsWatermarkFunc(metrics, time.Now(), time.Hour*1).UTC())
}

func (ex *pubsubExporter) consumeLogs(ctx context.Context, logs pdata.Logs) error {
	buffer, err := ex.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, OtlpProtoLog, buffer, ex.logsWatermarkFunc(logs, time.Now(), time.Hour*1).UTC())
}
