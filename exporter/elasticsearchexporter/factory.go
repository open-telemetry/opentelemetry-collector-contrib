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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	defaultLogsIndex    = "logs-generic-default"
	defaultMetricsIndex = "metrics-generic-default"
	defaultTracesIndex  = "traces-generic-default"
)

var defaultBatcherMinSizeItems = 5000

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
	qs := exporterhelper.NewDefaultQueueConfig()
	qs.Enabled = false

	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 90 * time.Second
	httpClientConfig.Compression = configcompression.TypeGzip
	httpClientConfig.CompressionParams.Level = gzip.BestSpeed

	return &Config{
		QueueSettings: qs,
		ClientConfig:  httpClientConfig,
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
			LogRequestBody:  false,
			LogResponseBody: false,
		},
		Batcher: BatcherConfig{
			Config: exporterbatcher.Config{
				FlushTimeout: 30 * time.Second,
				SizeConfig: exporterbatcher.SizeConfig{
					Sizer:   exporterbatcher.SizerTypeItems,
					MinSize: defaultBatcherMinSizeItems,
				},
			},
		},
		Flush: FlushSettings{
			Bytes:    5e+6,
			Interval: 30 * time.Second,
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

	handleDeprecatedConfig(cf, set.Logger)

	exporter := newExporter(cf, set, cf.LogsIndex, cf.LogsDynamicIndex.Enabled)

	return exporterhelper.NewLogs(
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
	handleDeprecatedConfig(cf, set.Logger)

	exporter := newExporter(cf, set, cf.MetricsIndex, cf.MetricsDynamicIndex.Enabled)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exporter.pushMetricsData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
	)
}

func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	cf := cfg.(*Config)
	handleDeprecatedConfig(cf, set.Logger)

	exporter := newExporter(cf, set, cf.TracesIndex, cf.TracesDynamicIndex.Enabled)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
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

	exporter := newExporter(cf, set, "", false)

	return xexporterhelper.NewProfilesExporter(
		ctx,
		set,
		cfg,
		exporter.pushProfilesData,
		exporterhelperOptions(cf, exporter.Start, exporter.Shutdown)...,
	)
}

func exporterhelperOptions(
	cfg *Config,
	start component.StartFunc,
	shutdown component.ShutdownFunc,
) []exporterhelper.Option {
	opts := []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
		exporterhelper.WithQueue(cfg.QueueSettings),
	}
	if cfg.Batcher.enabledSet {
		opts = append(opts, exporterhelper.WithBatcher(cfg.Batcher.Config))

		// Effectively disable timeout_sender because timeout is enforced in bulk indexer.
		//
		// We keep timeout_sender enabled in the async mode (Batcher.Enabled == nil),
		// to ensure sending data to the background workers will not block indefinitely.
		opts = append(opts, exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}))
	}
	return opts
}
