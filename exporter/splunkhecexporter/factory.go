// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"
)

const (
	defaultMaxIdleCons          = 100
	defaultHTTPTimeout          = 10 * time.Second
	defaultHTTP2ReadIdleTimeout = time.Second * 10
	defaultHTTP2PingTimeout     = time.Second * 10
	defaultIdleConnTimeout      = 10 * time.Second
	defaultSplunkAppName        = "OpenTelemetry Collector Contrib"
)

type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

type baseTracesExporter struct {
	component.Component
	consumer.Traces
}

type baseProfilesExporter struct {
	component.Component
	xconsumer.Profiles
}

// NewFactory creates a factory for Splunk HEC exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithProfiles(createProfilesExporter, metadata.ProfilesStability))
}

func createDefaultConfig() component.Config {
	defaultMaxConns := defaultMaxIdleCons
	defaultIdleConnTimeout := defaultIdleConnTimeout

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = defaultHTTPTimeout
	clientConfig.IdleConnTimeout = defaultIdleConnTimeout
	clientConfig.MaxIdleConnsPerHost = defaultMaxConns
	clientConfig.MaxIdleConns = defaultMaxConns
	clientConfig.HTTP2ReadIdleTimeout = defaultHTTP2ReadIdleTimeout
	clientConfig.HTTP2PingTimeout = defaultHTTP2PingTimeout

	return &Config{
		LogDataEnabled:          true,
		ProfilingDataEnabled:    true,
		ClientConfig:            clientConfig,
		SplunkAppName:           defaultSplunkAppName,
		BackOffConfig:           configretry.NewDefaultBackOffConfig(),
		QueueSettings:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		DisableCompression:      false,
		MaxContentLengthLogs:    defaultContentLengthLogsLimit,
		MaxContentLengthMetrics: defaultContentLengthMetricsLimit,
		MaxContentLengthTraces:  defaultContentLengthTracesLimit,
		MaxEventSize:            defaultMaxEventSize,
		OtelAttrsToHec:          translator.DefaultHecToOtelAttrs(),
		HecFields:               translator.DefaultOtelToHecFields(),
		HealthPath:              splunk.DefaultHealthPath,
		HecHealthCheckEnabled:   false,
		ExportRaw:               false,
		Telemetry: HecTelemetry{
			Enabled:              false,
			OverrideMetricsNames: map[string]string{},
			ExtraAttributes:      map[string]string{},
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	cfg := config.(*Config)
	c := newTracesClient(set, cfg)

	e, err := exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		c.pushTraceData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(hecQueueSettings(cfg.QueueSettings)),
		exporterhelper.WithStart(c.start),
		exporterhelper.WithShutdown(c.stop),
	)
	if err != nil {
		return nil, err
	}

	wrapped := &baseTracesExporter{
		Component: e,
		Traces: batchperresourceattr.NewMultiBatchPerResourceTraces(
			[]string{splunk.HecTokenLabel, splunk.DefaultIndexLabel},
			e,
			batchperresourceattr.WithMetadataInjection(),
		),
	}

	return wrapped, nil
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)
	c := newMetricsClient(set, cfg)

	e, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		c.pushMetricsData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(hecQueueSettings(cfg.QueueSettings)),
		exporterhelper.WithStart(c.start),
		exporterhelper.WithShutdown(c.stop),
	)
	if err != nil {
		return nil, err
	}

	wrapped := &baseMetricsExporter{
		Component: e,
		Metrics: batchperresourceattr.NewMultiBatchPerResourceMetrics(
			[]string{splunk.HecTokenLabel, splunk.DefaultIndexLabel},
			e,
			batchperresourceattr.WithMetadataInjection(),
		),
	}

	return wrapped, nil
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter exporter.Logs, err error) {
	cfg := config.(*Config)
	c := newLogsClient(set, cfg)

	logsExporter, err := exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		c.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(hecQueueSettings(cfg.QueueSettings)),
		exporterhelper.WithStart(c.start),
		exporterhelper.WithShutdown(c.stop),
	)
	if err != nil {
		return nil, err
	}

	wrapped := &baseLogsExporter{
		Component: logsExporter,
		Logs: batchperresourceattr.NewMultiBatchPerResourceLogs(
			[]string{splunk.HecTokenLabel, splunk.DefaultIndexLabel},
			&perScopeBatcher{
				logsEnabled:      cfg.LogDataEnabled,
				profilingEnabled: cfg.ProfilingDataEnabled,
				logger:           set.Logger,
				next:             logsExporter,
			},
			batchperresourceattr.WithMetadataInjection(),
		),
	}

	return wrapped, nil
}

func createProfilesExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter xexporter.Profiles, err error) {
	cfg := config.(*Config)
	c := newProfilesClient(set, cfg)

	profilesExporter, err := xexporterhelper.NewProfiles(
		ctx,
		set,
		cfg,
		c.pushProfilesData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(hecQueueSettings(cfg.QueueSettings)),
		exporterhelper.WithStart(c.start),
		exporterhelper.WithShutdown(c.stop),
	)
	if err != nil {
		return nil, err
	}

	wrapped := &baseProfilesExporter{
		Component: profilesExporter,
		Profiles: batchperresourceattr.NewMultiBatchPerResourceProfiles(
			[]string{splunk.HecTokenLabel, splunk.DefaultIndexLabel},
			profilesExporter,
			batchperresourceattr.WithMetadataInjection(),
		),
	}

	return wrapped, nil
}

// hecRequiredMetadataKeys are the context metadata keys the exporterhelper
// batcher must use to partition requests so different HEC routing targets
// are never merged into the same batch.
var hecRequiredMetadataKeys = []string{splunk.HecTokenLabel, splunk.DefaultIndexLabel}

// hecQueueSettings returns a copy of qs with hecRequiredMetadataKeys
// guaranteed to be present in Batch.Partition.MetadataKeys. Keys already
// set by the user are preserved; required keys are appended only when absent
// (case-insensitive). Returns qs unchanged when batching is not configured.
func hecQueueSettings(qs configoptional.Optional[exporterhelper.QueueBatchConfig]) configoptional.Optional[exporterhelper.QueueBatchConfig] {
	if !qs.HasValue() || !qs.Get().Batch.HasValue() {
		return qs
	}
	qCopy := *qs.Get()
	bCopy := *qCopy.Batch.Get()

	existing := make(map[string]bool, len(bCopy.Partition.MetadataKeys))
	for _, k := range bCopy.Partition.MetadataKeys {
		existing[strings.ToLower(k)] = true
	}
	for _, k := range hecRequiredMetadataKeys {
		if !existing[strings.ToLower(k)] {
			bCopy.Partition.MetadataKeys = append(bCopy.Partition.MetadataKeys, k)
		}
	}
	qCopy.Batch = configoptional.Some(bCopy)
	return configoptional.Some(qCopy)
}
