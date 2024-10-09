// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

var retryOn429FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.prometheusremotewritexporter.RetryOn429",
	featuregate.StageAlpha,
	featuregate.WithRegisterFromVersion("v0.101.0"),
	featuregate.WithRegisterDescription("When enabled, the Prometheus remote write exporter will retry 429 http status code. Requires exporter.prometheusremotewritexporter.metrics.RetryOn429 to be enabled."),
)

// NewFactory creates a new Prometheus Remote Write exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability))
}

func createMetricsExporter(ctx context.Context, set exporter.Settings,
	cfg component.Config) (exporter.Metrics, error) {

	prwCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	if prwCfg.RemoteWriteQueue.NumConsumers != 0 {
		set.Logger.Warn("Currently, remote_write_queue.num_consumers doesn't have any effect due to incompatibility with Prometheus remote write API. The value will be ignored. Please see https://github.com/open-telemetry/opentelemetry-collector/issues/2949 for more information.")
	}

	prwe, err := newPRWExporter(prwCfg, set)
	if err != nil {
		return nil, err
	}

	// Don't allow users to configure the queue.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/2949.
	// Prometheus remote write samples needs to be in chronological
	// order for each timeseries. If we shard the incoming metrics
	// without considering this limitation, we experience
	// "out of order samples" errors.
	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		prwe.PushMetrics,
		exporterhelper.WithTimeout(prwCfg.TimeoutSettings),
		exporterhelper.WithQueue(exporterhelper.QueueConfig{
			Enabled:      prwCfg.RemoteWriteQueue.Enabled,
			NumConsumers: 1,
			QueueSize:    prwCfg.RemoteWriteQueue.QueueSize,
		}),
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

	return &Config{
		Namespace:         "",
		ExternalLabels:    map[string]string{},
		MaxBatchSizeBytes: 3000000,
		TimeoutSettings:   exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:     retrySettings,
		AddMetricSuffixes: true,
		SendMetadata:      false,
		ClientConfig:      clientConfig,
		// TODO(jbd): Adjust the default queue size.
		RemoteWriteQueue: RemoteWriteQueue{
			Enabled:      true,
			QueueSize:    10000,
			NumConsumers: 5,
		},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		CreatedMetric: &CreatedMetric{
			Enabled: false,
		},
	}
}
