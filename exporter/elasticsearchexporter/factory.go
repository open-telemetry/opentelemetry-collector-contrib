// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"compress/gzip"
	"context"
	"maps"
	"net/http"
	"slices"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

var defaultBatcherMinSizeItems = int64(5000)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithProfiles(createProfilesExporter, metadata.ProfilesStability),
	)
}

func createDefaultConfig() component.Config {
	// TODO(lahsivjar): This is deviating from the original defaults:
	// - block_on_overflow: by default this is set to `false` i.e. clients will get
	//   retryable error when queue is full. However, the original behaviour is
	//   that we will block until a consumer is free ([ref](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/58308d77fa47e74bd8ed402ef4cd944cc2a4126a/exporter/elasticsearchexporter/bulkindexer.go#L325-L329))
	qs := exporterhelper.NewDefaultQueueConfig()
	qs.QueueSize = 10
	qs.Batch = configoptional.Some(exporterhelper.BatchConfig{
		FlushTimeout: 10 * time.Second,
		MinSize:      5e+6,
		MaxSize:      10e+6,
		Sizer:        exporterhelper.RequestSizerTypeBytes,
	})

	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 90 * time.Second
	httpClientConfig.Compression = configcompression.TypeGzip
	httpClientConfig.CompressionParams.Level = gzip.BestSpeed

	return &Config{
		QueueBatchConfig: qs,
		ClientConfig:     httpClientConfig,
		LogsDynamicID: DynamicIDSettings{
			Enabled: false,
		},
		LogsDynamicPipeline: DynamicPipelineSettings{
			Enabled: false,
		},
		Retry: RetrySettings{
			Enabled:         true,
			MaxRetries:      0, // default is set in exporter code
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
			RetryOnStatus: []int{
				http.StatusTooManyRequests,
			},
		},
		Mapping: MappingsSettings{
			Mode:         "otel",
			AllowedModes: slices.Sorted(maps.Keys(canonicalMappingModes)),
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
		TelemetrySettings: TelemetrySettings{
			LogRequestBody:              false,
			LogResponseBody:             false,
			LogFailedDocsInput:          false,
			LogFailedDocsInputRateLimit: time.Second,
		},
		IncludeSourceOnError: nil,
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

	handleDeprecatedConfig(cf, set.Logger)
	handleTelemetryConfig(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.LogsIndex)
	if err != nil {
		return nil, err
	}

	qbs := exporterhelper.NewLogsQueueBatchSettings()
	if len(cf.MetadataKeys) > 0 {
		qbs.Partitioner = metadataKeysPartitioner{keys: cf.MetadataKeys}
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown, qbs)...,
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	cf := cfg.(*Config)
	handleDeprecatedConfig(cf, set.Logger)
	handleTelemetryConfig(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.MetricsIndex)
	if err != nil {
		return nil, err
	}

	qbs := exporterhelper.NewMetricsQueueBatchSettings()
	if len(cf.MetadataKeys) > 0 {
		qbs.Partitioner = metadataKeysPartitioner{keys: cf.MetadataKeys}
	}

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown, qbs)...,
	)
}

func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	cf := cfg.(*Config)
	handleDeprecatedConfig(cf, set.Logger)
	handleTelemetryConfig(cf, set.Logger)

	exporter, err := newExporter(cf, set, cf.TracesIndex)
	if err != nil {
		return nil, err
	}

	qbs := exporterhelper.NewTracesQueueBatchSettings()
	if len(cf.MetadataKeys) > 0 {
		qbs.Partitioner = metadataKeysPartitioner{keys: cf.MetadataKeys}
	}

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown, qbs)...,
	)
}

// createProfilesExporter creates a new exporter for profiles.
//
// Profiles are directly indexed into Elasticsearch.
func createProfilesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (xexporter.Profiles, error) {
	cf := cfg.(*Config)

	handleDeprecatedConfig(cf, set.Logger)
	handleTelemetryConfig(cf, set.Logger)

	exporter, err := newExporter(cf, set, "")
	if err != nil {
		return nil, err
	}

	qbs := xexporterhelper.NewProfilesQueueBatchSettings()
	if len(cf.MetadataKeys) > 0 {
		qbs.Partitioner = metadataKeysPartitioner{keys: cf.MetadataKeys}
	}

	return xexporterhelper.NewProfiles(
		ctx,
		set,
		cfg,
		exporter.pushProfilesData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown, qbs)...,
	)
}

func exporterhelperOptions(
	cfg *Config,
	start component.StartFunc,
	shutdown component.ShutdownFunc,
	qbs exporterhelper.QueueBatchSettings,
) []exporterhelper.Option {
	// not setting capabilities as they will default to non-mutating but will be updated
	// by the base-exporter to mutating if batching is enabled.
	opts := []exporterhelper.Option{
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
		exporterhelper.WithQueueBatch(cfg.QueueBatchConfig, qbs),
	}

	// Effectively disable timeout_sender because timeout is enforced in bulk indexer.
	//
	// We keep timeout_sender enabled in the async mode (sending_queue not enabled OR sending
	// queue enabled but batching not enabled OR based on the deprecated batcher setting), to
	// ensure sending data to the background workers will not block indefinitely.
	if cfg.QueueBatchConfig.Enabled {
		opts = append(opts, exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}))
	}
	return opts
}
