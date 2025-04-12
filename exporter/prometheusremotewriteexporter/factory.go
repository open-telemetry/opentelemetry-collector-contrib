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

var enableMultipleWorkersFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.prometheusremotewritexporter.EnableMultipleWorkers",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled and settings configured, the Prometheus remote exporter will"+
		" spawn multiple workers/goroutines to handle incoming metrics batches concurrently"),
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

	if !enableMultipleWorkersFeatureGate.IsEnabled() && prwCfg.RemoteWriteQueue.NumConsumers != 5 {
		set.Logger.Warn("`remote_write_queue.num_consumers` will be used to configure processing parallelism, rather than request parallelism in a future release. This may cause out-of-order issues unless you take action. Please migrate to using `max_batch_request_parallelism` to keep the your existing behavior.")
	}

	prwe, err := newPRWExporter(prwCfg, set)
	if err != nil {
		return nil, err
	}

	numConsumers := 1
	if enableMultipleWorkersFeatureGate.IsEnabled() {
		numConsumers = prwCfg.RemoteWriteQueue.NumConsumers
	}
	exporter, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		prwe.PushMetrics,
		exporterhelper.WithTimeout(prwCfg.TimeoutSettings),
		exporterhelper.WithQueue(exporterhelper.QueueBatchConfig{
			Enabled:      prwCfg.RemoteWriteQueue.Enabled,
			NumConsumers: numConsumers,
			QueueSize:    int64(prwCfg.RemoteWriteQueue.QueueSize),
			Sizer:        exporterhelper.RequestSizerTypeRequests,
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

	numConsumers := 5
	if enableMultipleWorkersFeatureGate.IsEnabled() {
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
		SendMetadata:      false,
		ClientConfig:      clientConfig,
		// TODO(jbd): Adjust the default queue size.
		RemoteWriteQueue: RemoteWriteQueue{
			Enabled:      true,
			QueueSize:    10000,
			NumConsumers: numConsumers,
		},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
	}
}
