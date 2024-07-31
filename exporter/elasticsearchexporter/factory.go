// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	defaultLogsIndex    = "logs-generic-default"
	defaultMetricsIndex = "metrics-generic-default"
	defaultTracesIndex  = "traces-generic-default"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 90 * time.Second

	return &Config{
		QueueSettings: qs,
		ClientConfig:  httpClientConfig,
		Index:         "",
		LogsIndex:     defaultLogsIndex,
		LogsDynamicIndex: DynamicIndexSetting{
			Enabled: false,
		},
		MetricsIndex: defaultMetricsIndex,
		MetricsDynamicIndex: DynamicIndexSetting{
			Enabled: true,
		},
		TracesIndex: defaultTracesIndex,
		TracesDynamicIndex: DynamicIndexSetting{
			Enabled: false,
		},
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
			RetryOnStatus: []int{
				http.StatusTooManyRequests,
				http.StatusInternalServerError,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout,
			},
		},
		Mapping: MappingsSettings{
			Mode:  "none",
			Dedot: true,
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
		TelemetrySettings: TelemetrySettings{
			LogRequestBody:  false,
			LogResponseBody: false,
		},
		Batcher: BatcherConfig{
			FlushTimeout: 30 * time.Second,
			MinSizeConfig: exporterbatcher.MinSizeConfig{
				MinSizeItems: 5000,
			},
			MaxSizeConfig: exporterbatcher.MaxSizeConfig{
				MaxSizeItems: 10000,
			},
		},
	}
}

// createLogsExporter creates a new exporter for logs.
//
// Logs are directly indexed into Elasticsearch.
func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	cf := cfg.(*Config)

	index := cf.LogsIndex
	if cf.Index != "" {
		set.Logger.Warn("index option are deprecated and replaced with logs_index and traces_index.")
		index = cf.Index
	}
	logConfigDeprecationWarnings(cf, set.Logger)

	exporter, err := newExporter(cf, set, index, cf.LogsDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	cf := cfg.(*Config)
	logConfigDeprecationWarnings(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.MetricsIndex, cf.MetricsDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
	)
}

func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config) (exporter.Traces, error) {

	cf := cfg.(*Config)
	logConfigDeprecationWarnings(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.TracesIndex, cf.TracesDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
	)
}

func exporterhelperOptions(
	cfg *Config,
	start component.StartFunc,
	shutdown component.ShutdownFunc,
) []exporterhelper.Option {
	opts := []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
		exporterhelper.WithQueue(cfg.QueueSettings),
	}
	if cfg.Batcher.Enabled != nil {
		batcherConfig := exporterbatcher.Config{
			Enabled:       *cfg.Batcher.Enabled,
			FlushTimeout:  cfg.Batcher.FlushTimeout,
			MinSizeConfig: cfg.Batcher.MinSizeConfig,
			MaxSizeConfig: cfg.Batcher.MaxSizeConfig,
		}
		opts = append(opts, exporterhelper.WithBatcher(batcherConfig))

		// Effectively disable timeout_sender because timeout is enforced in bulk indexer.
		//
		// We keep timeout_sender enabled in the async mode (Batcher.Enabled == nil),
		// to ensure sending data to the background workers will not block indefinitely.
		opts = append(opts, exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}))
	}
	return opts
}
