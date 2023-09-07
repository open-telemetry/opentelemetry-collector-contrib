// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"time"

	pubsub "cloud.google.com/go/pubsub/apiv1"
	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const name = "googlecloudpubsub"

type pubsubExporter struct {
	logger               *zap.Logger
	client               *pubsub.PublisherClient
	cancel               context.CancelFunc
	userAgent            string
	ceSource             string
	ceCompression        compression
	config               *Config
	tracesMarshaler      ptrace.Marshaler
	tracesWatermarkFunc  tracesWatermarkFunc
	metricsMarshaler     pmetric.Marshaler
	metricsWatermarkFunc metricsWatermarkFunc
	logsMarshaler        plog.Marshaler
	logsWatermarkFunc    logsWatermarkFunc
}

func (*pubsubExporter) Name() string {
	return name
}

type encoding int

const (
	otlpProtoTrace  encoding = iota
	otlpProtoMetric          = iota
	otlpProtoLog             = iota
)

type compression int

const (
	uncompressed compression = iota
	gZip                     = iota
)

type WatermarkBehavior int

const (
	current  WatermarkBehavior = iota
	earliest                   = iota
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
	if ex.config.endpoint != "" {
		if ex.config.insecure {
			var dialOpts []grpc.DialOption
			if ex.userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(ex.userAgent))
			}
			conn, _ := grpc.Dial(ex.config.endpoint, append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))...)
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(ex.config.endpoint))
		}
	}
	return copts
}

func (ex *pubsubExporter) publishMessage(ctx context.Context, encoding encoding, data []byte, watermark time.Time) error {
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
	case otlpProtoTrace:
		attributes["ce-type"] = "org.opentelemetry.otlp.traces.v1"
		attributes["content-type"] = "application/protobuf"
	case otlpProtoMetric:
		attributes["ce-type"] = "org.opentelemetry.otlp.metrics.v1"
		attributes["content-type"] = "application/protobuf"
	case otlpProtoLog:
		attributes["ce-type"] = "org.opentelemetry.otlp.logs.v1"
		attributes["content-type"] = "application/protobuf"
	}
	if ex.ceCompression == gZip {
		attributes["content-encoding"] = "gzip"
		data, err = ex.compress(data)
		if err != nil {
			return err
		}
	}
	_, err = ex.client.Publish(ctx, &pubsubpb.PublishRequest{
		Topic: ex.config.Topic,
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
	if ex.ceCompression == gZip {
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

func (ex *pubsubExporter) consumeTraces(ctx context.Context, traces ptrace.Traces) error {
	buffer, err := ex.tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, otlpProtoTrace, buffer, ex.tracesWatermarkFunc(traces, time.Now(), ex.config.Watermark.AllowedDrift).UTC())
}

func (ex *pubsubExporter) consumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	buffer, err := ex.metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, otlpProtoMetric, buffer, ex.metricsWatermarkFunc(metrics, time.Now(), ex.config.Watermark.AllowedDrift).UTC())
}

func (ex *pubsubExporter) consumeLogs(ctx context.Context, logs plog.Logs) error {
	buffer, err := ex.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}
	return ex.publishMessage(ctx, otlpProtoLog, buffer, ex.logsWatermarkFunc(logs, time.Now(), ex.config.Watermark.AllowedDrift).UTC())
}
