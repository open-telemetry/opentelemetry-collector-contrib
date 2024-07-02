// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

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
	qs.NumConsumers = 100 // default is too small as it also sets batch sender concurrency limit

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
		BatcherConfig: exporterbatcher.Config{
			Enabled:      true,
			FlushTimeout: 30 * time.Second,
			MinSizeConfig: exporterbatcher.MinSizeConfig{
				MinSizeItems: 5000,
			},
			MaxSizeConfig: exporterbatcher.MaxSizeConfig{
				MaxSizeItems: 10000,
			},
		},
		Mapping: MappingsSettings{
			Mode:  "none",
			Dedup: true,
			Dedot: true,
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
		NumWorkers: runtime.NumCPU(),
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

	if err := handleDeprecations(cf, set.Logger); err != nil {
		return nil, err
	}
	logConfigDeprecationWarnings(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.LogsIndex, cf.LogsDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithBatcher(cf.BatcherConfig),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
		exporterhelper.WithTimeout(getTimeoutConfig()),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	cf := cfg.(*Config)
	logConfigDeprecationWarnings(cf, set.Logger)

	// Workaround to avoid rejections from Elasticsearch.
	// TSDB does not accept 2 documents with the same timestamp and dimensions.
	//
	// Setting MaxSizeItems ensures that the batcher will not split a set of
	// metrics into multiple batches, potentially sending two metric data points
	// with the same timestamp and dimensions as separate documents.
	cf.BatcherConfig.MaxSizeConfig.MaxSizeItems = 0
	set.Logger.Warn("batcher.max_size_items is ignored: metrics exporter does not support batch splitting")

	exporter, err := newExporter(cf, set, cf.MetricsIndex, cf.MetricsDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithBatcher(cf.BatcherConfig),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
		exporterhelper.WithTimeout(getTimeoutConfig()),
	)
}

// createTracesExporter creates a new exporter for traces.
func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config) (exporter.Traces, error) {

	cf := cfg.(*Config)
	logConfigDeprecationWarnings(cf, set.Logger)

	if err := handleDeprecations(cf, set.Logger); err != nil {
		return nil, err
	}

	exporter, err := newExporter(cf, set, cf.TracesIndex, cf.TracesDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithBatcher(cf.BatcherConfig),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
		exporterhelper.WithTimeout(getTimeoutConfig()),
	)
}

func getTimeoutConfig() exporterhelper.TimeoutSettings {
	return exporterhelper.TimeoutSettings{
		Timeout: time.Duration(0), // effectively disable timeout_sender because timeout is enforced in bulk indexer
	}
}

// handleDeprecations handles deprecated config options.
// If possible, translate deprecated config options to new config options
// Otherwise, return an error so that the user is aware of an unsupported option.
func handleDeprecations(cf *Config, logger *zap.Logger) error { //nolint:unparam
	if cf.Index != "" {
		logger.Warn(`"index" option is deprecated and replaced with "logs_index" and "traces_index". Setting "logs_index" to the value of "index".`, zap.String("value", cf.Index))
		cf.LogsIndex = cf.Index
	}

	if cf.Flush.Bytes != 0 {
		const factor = 1000
		val := cf.Flush.Bytes / factor
		logger.Warn(fmt.Sprintf(`"flush.bytes" option is deprecated and replaced with "batcher.min_size_items". Setting "batcher.min_size_items" to the value of "flush.bytes" / %d.`, factor), zap.Int("value", val))
		cf.BatcherConfig.MinSizeItems = val
	}

	if cf.Flush.Interval != 0 {
		logger.Warn(`"flush.interval" option is deprecated and replaced with "batcher.flush_timeout". Setting "batcher.flush_timeout" to the value of "flush.interval".`, zap.Duration("value", cf.Flush.Interval))
		cf.BatcherConfig.FlushTimeout = cf.Flush.Interval
	}

	return nil
}
