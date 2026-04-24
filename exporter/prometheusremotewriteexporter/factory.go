// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"errors"
	"time"

	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// NewFactory creates a new Prometheus Remote Write exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability))
}

func createMetricsExporter(ctx context.Context, set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	prwCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	if !metadata.ExporterPrometheusremotewritexporterEnableMultipleWorkersFeatureGate.IsEnabled() && prwCfg.RemoteWriteQueue.NumConsumers != 5 {
		set.Logger.Warn("`remote_write_queue.num_consumers` will be used to configure processing parallelism, rather than request parallelism in a future release. This may cause out-of-order issues unless you take action. Please migrate to using `max_batch_request_parallelism` to keep the your existing behavior.")
	}

	if !prwCfg.AddMetricSuffixes {
		set.Logger.Warn("`add_metric_suffixes` is deprecated. Please use `translation_strategy: UnderscoreEscapingWithoutSuffixes` instead.")
	}

	prwe, err := newPRWExporter(prwCfg, set)
	if err != nil {
		return nil, err
	}

	numConsumers := 1
	if metadata.ExporterPrometheusremotewritexporterEnableMultipleWorkersFeatureGate.IsEnabled() {
		numConsumers = prwCfg.RemoteWriteQueue.NumConsumers
	}

	qCfg := configoptional.Default(exporterhelper.QueueBatchConfig{
		NumConsumers: numConsumers,
		QueueSize:    int64(prwCfg.RemoteWriteQueue.QueueSize),
		Sizer:        exporterhelper.RequestSizerTypeRequests,
	})
	if prwCfg.RemoteWriteQueue.Enabled {
		qCfg.GetOrInsertDefault()
	}

	exporter, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		prwe.PushMetrics,
		exporterhelper.WithTimeout(prwCfg.TimeoutSettings),
		exporterhelper.WithQueue(qCfg),
		exporterhelper.WithStart(prwe.Start),
		exporterhelper.WithShutdown(prwe.Shutdown),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(prwCfg.ResourceToTelemetrySettings, exporter), nil
}

func createDefaultConfig() component.Config {
	retrySettings := configretry.NewDefaultBackOffConfig()
	retrySettings.InitialInterval = 50 * time.Millisecond
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://some.url:9411/api/prom/push"
	// We almost read 0 bytes, so no need to tune ReadBufferSize.
	clientConfig.ReadBufferSize = 0
	clientConfig.WriteBufferSize = 512 * 1024
	clientConfig.Timeout = exporterhelper.NewDefaultTimeoutConfig().Timeout

	numConsumers := 5
	if metadata.ExporterPrometheusremotewritexporterEnableMultipleWorkersFeatureGate.IsEnabled() {
		numConsumers = 1
	}
	return &Config{
		Namespace:         "",
		ExternalLabels:    map[string]string{},
		MaxBatchSizeBytes: 3000000,
		// To set this as default once `exporter.prometheusremotewritexporter.EnableMultipleWorkers` is removed
		// MaxBatchRequestParallelism: 5,
		TimeoutSettings:   exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:     retrySettings,
		AddMetricSuffixes: true,
		// TODO: Set TranslationStrategy to UnderscoreEscapingWithSuffixes once AddMetricSuffixes is removed.
		SendMetadata:        false,
		RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
		ClientConfig:        clientConfig,
		// TODO(jbd): Adjust the default queue size.
		RemoteWriteQueue: RemoteWriteQueue{
			Enabled:      true,
			QueueSize:    10000,
			NumConsumers: numConsumers,
		},
		TargetInfo: TargetInfo{
			Enabled: true,
		},
	}
}
