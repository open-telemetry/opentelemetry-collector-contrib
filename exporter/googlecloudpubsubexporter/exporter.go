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

func (ex *pubsubExporter) getMessageAttributes(encoding encoding, watermark time.Time) (map[string]string, error) {
	id, err := ex.makeUUID()
	if err != nil {
		return nil, err
	}
	ceTime, err := watermark.MarshalText()
	if err != nil {
		return nil, err
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
	}
	return attributes, err
}

func (ex *pubsubExporter) consumeTraces(ctx context.Context, traces ptrace.Traces) error {
	if !ex.config.Ordering.Enabled {
		return ex.publishTraces(ctx, traces, "")
	}

	tracesByOrderingKey := map[string]ptrace.Traces{
		"": ptrace.NewTraces(),
	}
	traces.ResourceSpans().RemoveIf(func(resourceSpans ptrace.ResourceSpans) bool {
		orderingKey, found := resourceSpans.Resource().Attributes().Get(ex.config.Ordering.FromResourceAttribute)
		if !found {
			return false
		}

		orderingKeyValue := orderingKey.AsString()
		if _, exists := tracesByOrderingKey[orderingKeyValue]; !exists {
			tracesByOrderingKey[orderingKeyValue] = ptrace.NewTraces()
		}

		if ex.config.Ordering.RemoveResourceAttribute {
			_ = resourceSpans.Resource().Attributes().Remove(ex.config.Ordering.FromResourceAttribute)
		}

		resourceSpans.MoveTo(tracesByOrderingKey[orderingKeyValue].ResourceSpans().AppendEmpty())
		return true
	})

	// No ordering key
	if traces.SpanCount() > 0 {
		traces.ResourceSpans().MoveAndAppendTo(tracesByOrderingKey[""].ResourceSpans())
	}

	for key, tracesForKey := range tracesByOrderingKey {
		if err := ex.publishTraces(ctx, tracesForKey, key); err != nil {
			return err
		}
	}
	return nil
}

func (ex *pubsubExporter) publishTraces(ctx context.Context, tracesForKey ptrace.Traces, orderingKey string) error {
	watermark := ex.tracesWatermarkFunc(tracesForKey, time.Now(), ex.config.Watermark.AllowedDrift).UTC()
	attributes, attributesErr := ex.getMessageAttributes(otlpProtoTrace, watermark)
	if attributesErr != nil {
		return fmt.Errorf("error while preparing pubsub message attributes: %w", attributesErr)
	}

	data, err := ex.tracesMarshaler.MarshalTraces(tracesForKey)
	if err != nil {
		return fmt.Errorf("error while marshaling traces: %w", err)
	}

	return ex.publishMessage(ctx, data, attributes, orderingKey)
}

func (ex *pubsubExporter) consumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	if !ex.config.Ordering.Enabled {
		return ex.publishMetrics(ctx, metrics, "")
	}

	metricsByOrderingKey := map[string]pmetric.Metrics{
		"": pmetric.NewMetrics(),
	}
	metrics.ResourceMetrics().RemoveIf(func(resourceMetrics pmetric.ResourceMetrics) bool {
		orderingKey, found := resourceMetrics.Resource().Attributes().Get(ex.config.Ordering.FromResourceAttribute)
		if !found {
			return false
		}

		orderingKeyValue := orderingKey.AsString()
		if _, exists := metricsByOrderingKey[orderingKeyValue]; !exists {
			metricsByOrderingKey[orderingKeyValue] = pmetric.NewMetrics()
		}

		if ex.config.Ordering.RemoveResourceAttribute {
			_ = resourceMetrics.Resource().Attributes().Remove(ex.config.Ordering.FromResourceAttribute)
		}

		resourceMetrics.MoveTo(metricsByOrderingKey[orderingKeyValue].ResourceMetrics().AppendEmpty())
		return true
	})

	// No ordering key
	if metrics.DataPointCount() > 0 {
		metrics.ResourceMetrics().MoveAndAppendTo(metricsByOrderingKey[""].ResourceMetrics())
	}

	for key, metricsForKey := range metricsByOrderingKey {
		if err := ex.publishMetrics(ctx, metricsForKey, key); err != nil {
			return err
		}
	}
	return nil
}

func (ex *pubsubExporter) publishMetrics(ctx context.Context, metricsForKey pmetric.Metrics, orderingKey string) error {
	watermark := ex.metricsWatermarkFunc(metricsForKey, time.Now(), ex.config.Watermark.AllowedDrift).UTC()
	attributes, attributesErr := ex.getMessageAttributes(otlpProtoMetric, watermark)
	if attributesErr != nil {
		return fmt.Errorf("error while preparing pubsub message attributes: %w", attributesErr)
	}

	data, err := ex.metricsMarshaler.MarshalMetrics(metricsForKey)
	if err != nil {
		return fmt.Errorf("error while marshaling metrics: %w", err)
	}

	return ex.publishMessage(ctx, data, attributes, orderingKey)
}

func (ex *pubsubExporter) consumeLogs(ctx context.Context, logs plog.Logs) error {
	if !ex.config.Ordering.Enabled {
		return ex.publishLogs(ctx, logs, "")
	}

	logsByOrderingKey := map[string]plog.Logs{
		"": plog.NewLogs(),
	}
	if ex.config.Ordering.Enabled {
		logs.ResourceLogs().RemoveIf(func(resourceLogs plog.ResourceLogs) bool {
			orderingKey, found := resourceLogs.Resource().Attributes().Get(ex.config.Ordering.FromResourceAttribute)
			if !found {
				return false
			}

			orderingKeyValue := orderingKey.AsString()
			if _, exists := logsByOrderingKey[orderingKeyValue]; !exists {
				logsByOrderingKey[orderingKeyValue] = plog.NewLogs()
			}

			if ex.config.Ordering.RemoveResourceAttribute {
				_ = resourceLogs.Resource().Attributes().Remove(ex.config.Ordering.FromResourceAttribute)
			}

			resourceLogs.MoveTo(logsByOrderingKey[orderingKeyValue].ResourceLogs().AppendEmpty())
			return true
		})
	}

	// No ordering key
	if logs.LogRecordCount() > 0 {
		logs.ResourceLogs().MoveAndAppendTo(logsByOrderingKey[""].ResourceLogs())
	}

	for key, logsForKey := range logsByOrderingKey {
		if err := ex.publishLogs(ctx, logsForKey, key); err != nil {
			return err
		}
	}
	return nil
}

func (ex *pubsubExporter) publishLogs(ctx context.Context, logs plog.Logs, orderingKey string) error {
	watermark := ex.logsWatermarkFunc(logs, time.Now(), ex.config.Watermark.AllowedDrift).UTC()
	attributes, attributesErr := ex.getMessageAttributes(otlpProtoLog, watermark)
	if attributesErr != nil {
		return fmt.Errorf("error while preparing pubsub message attributes: %w", attributesErr)
	}

	data, err := ex.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return fmt.Errorf("error while marshaling logs: %w", err)
	}

	return ex.publishMessage(ctx, data, attributes, orderingKey)
}

func (ex *pubsubExporter) publishMessage(ctx context.Context, data []byte, attributes map[string]string, orderingKey string) error {
	if len(data) == 0 {
		return nil
	}

	data, compressErr := ex.compress(data)
	if compressErr != nil {
		return fmt.Errorf("error while compressing pubsub message payload: %w", compressErr)
	}

	_, publishErr := ex.client.Publish(ctx, &pubsubpb.PublishRequest{
		Topic: ex.config.Topic,
		Messages: []*pubsubpb.PubsubMessage{{
			Attributes:  attributes,
			OrderingKey: orderingKey,
			Data:        data,
		}},
	})
	if publishErr != nil {
		return fmt.Errorf("failed to publish pubsub message for ordering key %q: %w", orderingKey, publishErr)
	}
	return nil
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
