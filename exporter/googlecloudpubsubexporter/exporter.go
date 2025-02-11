// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type pubsubExporter struct {
	logger               *zap.Logger
	client               publisherClient
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

	// To be overridden in tests
	makeUUID   func() (uuid.UUID, error)
	makeClient func(ctx context.Context, cfg *Config, userAgent string) (publisherClient, error)
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
		client, err := ex.makeClient(ctx, ex.config, ex.userAgent)
		if err != nil {
			return fmt.Errorf("failed creating the gRPC client to Pubsub: %w", err)
		}

		ex.client = client
	}
	return nil
}

func (ex *pubsubExporter) shutdown(_ context.Context) error {
	if ex.client == nil {
		return nil
	}

	client := ex.client
	ex.client = nil
	return client.Close()
}

func (ex *pubsubExporter) publishMessage(ctx context.Context, encoding encoding, data []byte, watermark time.Time) error {
	if len(data) == 0 {
		return nil
	}

	id, err := ex.makeUUID()
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
