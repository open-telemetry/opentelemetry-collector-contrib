// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter/internal/metadata"
)

const (
	defaultTimeout = 12 * time.Second
)

// NewFactory creates a factory for Google Cloud Pub/Sub exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability))
}

var exporters = map[*Config]*pubsubExporter{}

func ensureExporter(params exporter.CreateSettings, pCfg *Config) *pubsubExporter {
	receiver := exporters[pCfg]
	if receiver != nil {
		return receiver
	}
	receiver = &pubsubExporter{
		logger:           params.Logger,
		userAgent:        strings.ReplaceAll(pCfg.UserAgent, "{{version}}", params.BuildInfo.Version),
		ceSource:         fmt.Sprintf("/opentelemetry/collector/%s/%s", name, params.BuildInfo.Version),
		config:           pCfg,
		tracesMarshaler:  &ptrace.ProtoMarshaler{},
		metricsMarshaler: &pmetric.ProtoMarshaler{},
		logsMarshaler:    &plog.ProtoMarshaler{},
	}
	// we ignore the error here as the config is already validated with the same method
	receiver.ceCompression, _ = pCfg.parseCompression()
	watermarkBehavior, _ := pCfg.Watermark.parseWatermarkBehavior()
	switch watermarkBehavior {
	case earliest:
		receiver.tracesWatermarkFunc = earliestTracesWatermark
		receiver.metricsWatermarkFunc = earliestMetricsWatermark
		receiver.logsWatermarkFunc = earliestLogsWatermark
	case current:
		receiver.tracesWatermarkFunc = currentTracesWatermark
		receiver.metricsWatermarkFunc = currentMetricsWatermark
		receiver.logsWatermarkFunc = currentLogsWatermark
	}
	exporters[pCfg] = receiver
	return receiver
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	return &Config{
		UserAgent:       "opentelemetry-collector-contrib {{version}}",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		Watermark: WatermarkConfig{
			Behavior:     "current",
			AllowedDrift: 0,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	pCfg := cfg.(*Config)
	pubsubExporter := ensureExporter(set, pCfg)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		pubsubExporter.consumeTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(pCfg.TimeoutSettings),
		exporterhelper.WithRetry(pCfg.RetrySettings),
		exporterhelper.WithQueue(pCfg.QueueSettings),
		exporterhelper.WithStart(pubsubExporter.start),
		exporterhelper.WithShutdown(pubsubExporter.shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {

	pCfg := cfg.(*Config)
	pubsubExporter := ensureExporter(set, pCfg)
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		pubsubExporter.consumeMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(pCfg.TimeoutSettings),
		exporterhelper.WithRetry(pCfg.RetrySettings),
		exporterhelper.WithQueue(pCfg.QueueSettings),
		exporterhelper.WithStart(pubsubExporter.start),
		exporterhelper.WithShutdown(pubsubExporter.shutdown),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Logs, error) {

	pCfg := cfg.(*Config)
	pubsubExporter := ensureExporter(set, pCfg)

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		pubsubExporter.consumeLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(pCfg.TimeoutSettings),
		exporterhelper.WithRetry(pCfg.RetrySettings),
		exporterhelper.WithQueue(pCfg.QueueSettings),
		exporterhelper.WithStart(pubsubExporter.start),
		exporterhelper.WithShutdown(pubsubExporter.shutdown),
	)
}
