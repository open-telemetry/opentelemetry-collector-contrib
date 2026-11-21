// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
)

// TODO: Find a place for this to be shared.
type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

// TODO: Find a place for this to be shared.
type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

// TODO: Find a place for this to be shared.
type baseTracesExporter struct {
	component.Component
	consumer.Traces
}

// NewFactory creates a factory for S3 exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	queueCfg := configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	timeoutCfg := exporterhelper.NewDefaultTimeoutConfig()

	return &Config{
		QueueSettings:   queueCfg,
		TimeoutSettings: timeoutCfg,
		S3Uploader: S3UploaderConfig{
			Region:            "us-east-1",
			S3PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			StorageClass:      "STANDARD",
			RetryMode:         DefaultRetryMode,
			RetryMaxAttempts:  DefaultRetryMaxAttempts,
			RetryMaxBackoff:   DefaultRetryMaxBackoff,
		},
		MarshalerName: "otlp_json",
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "logs", params)

	logsExporter, err := exporterhelper.NewLogs(ctx, params,
		config,
		s3Exporter.ConsumeLogs,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(s3QueueSettings(cfg)),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	routingKeys := cfg.ResourceAttrsToS3.routingKeys()
	if len(routingKeys) == 0 {
		return logsExporter, nil
	}

	wrapped := &baseLogsExporter{
		Component: logsExporter,
		Logs: batchperresourceattr.NewMultiBatchPerResourceLogs(
			routingKeys,
			logsExporter,
			batchperresourceattr.WithMetadataInjection(),
		),
	}
	return wrapped, nil
}

func createMetricsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "metrics", params)

	if config.(*Config).MarshalerName == SumoIC {
		return nil, errors.New("metrics are not supported by sumo_ic output format")
	}

	metricsExporter, err := exporterhelper.NewMetrics(ctx, params,
		config,
		s3Exporter.ConsumeMetrics,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(s3QueueSettings(cfg)),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	routingKeys := cfg.ResourceAttrsToS3.routingKeys()
	if len(routingKeys) == 0 {
		return metricsExporter, nil
	}

	wrapped := &baseMetricsExporter{
		Component: metricsExporter,
		Metrics: batchperresourceattr.NewMultiBatchPerResourceMetrics(
			routingKeys,
			metricsExporter,
			batchperresourceattr.WithMetadataInjection(),
		),
	}
	return wrapped, nil
}

func createTracesExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "traces", params)

	if config.(*Config).MarshalerName == SumoIC {
		return nil, errors.New("traces are not supported by sumo_ic output format")
	}

	tracesExporter, err := exporterhelper.NewTraces(ctx,
		params,
		config,
		s3Exporter.ConsumeTraces,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(s3QueueSettings(cfg)),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	routingKeys := cfg.ResourceAttrsToS3.routingKeys()
	if len(routingKeys) == 0 {
		return tracesExporter, nil
	}

	wrapped := &baseTracesExporter{
		Component: tracesExporter,
		Traces: batchperresourceattr.NewMultiBatchPerResourceTraces(
			routingKeys,
			tracesExporter,
			batchperresourceattr.WithMetadataInjection(),
		),
	}
	return wrapped, nil
}

// routingKeys returns the resource attribute keys that select the S3 bucket
// and prefix. Each request is split so that all its resources share the same
// values for these keys, because the upload takes them from the first resource.
func (r ResourceAttrsToS3) routingKeys() []string {
	var keys []string
	for _, k := range []string{r.S3Bucket, r.S3Prefix} {
		if k != "" && !containsFold(keys, k) {
			keys = append(keys, k)
		}
	}
	return keys
}

// s3QueueSettings returns the queue settings to use. When batching is enabled
// and the bucket or prefix comes from resource attributes, the batcher must
// also partition by those keys, or it merges requests that were split per
// bucket and prefix back into one, and the merged request is uploaded under
// the first resource's bucket and prefix.
func s3QueueSettings(cfg *Config) configoptional.Optional[exporterhelper.QueueBatchConfig] {
	qs := cfg.QueueSettings
	routingKeys := cfg.ResourceAttrsToS3.routingKeys()
	if len(routingKeys) == 0 || !qs.HasValue() || !qs.Get().Batch.HasValue() {
		return qs
	}
	qCopy := *qs.Get()
	bCopy := *qCopy.Batch.Get()
	keys := append([]string(nil), bCopy.Partition.MetadataKeys...)
	for _, k := range routingKeys {
		if !containsFold(keys, k) {
			keys = append(keys, k)
		}
	}
	bCopy.Partition.MetadataKeys = keys
	qCopy.Batch = configoptional.Some(bCopy)
	return configoptional.Some(qCopy)
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// checkAndCastConfig checks the configuration type and casts it to the S3 exporter Config struct.
func checkAndCastConfig(c component.Config) (*Config, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, errors.New("config structure is not of type *awss3exporter.Config")
	}
	return cfg, nil
}
