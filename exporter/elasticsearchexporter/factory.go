// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"compress/gzip"
	"context"
	"maps"
	"slices"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

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
	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 90 * time.Second
	httpClientConfig.Compression = configcompression.TypeGzip
	httpClientConfig.CompressionParams.Level = gzip.BestSpeed

	return &Config{
		ClientConfig: httpClientConfig,
		LogsDynamicID: DynamicIDSettings{
			Enabled: false,
		},
		LogsDynamicPipeline: DynamicPipelineSettings{
			Enabled: false,
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

	qbs := xexporterhelper.NewLogsQueueBatchSettings()
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

	qbs := xexporterhelper.NewMetricsQueueBatchSettings()
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

	qbs := xexporterhelper.NewTracesQueueBatchSettings()
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
	qbs xexporterhelper.QueueBatchSettings,
) []exporterhelper.Option {
	opts := []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: cfg.Timeout}),
	}
	qbc := cfg.QueueBatchConfig
	if qbc.Batch.HasValue() {
		opts = append(opts, xexporterhelper.WithQueueBatch(qbc, qbs))
	} else {
		opts = append(opts, exporterhelper.WithQueue(qbc))
	}
	return opts
}
