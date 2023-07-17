// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
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

func createMetricsExporter(ctx context.Context, set exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {

	prwCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
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
		exporterhelper.WithQueue(exporterhelper.QueueSettings{
			Enabled:      prwCfg.RemoteWriteQueue.Enabled,
			NumConsumers: 1,
			QueueSize:    prwCfg.RemoteWriteQueue.QueueSize,
		}),
		exporterhelper.WithRetry(prwCfg.RetrySettings),
		exporterhelper.WithStart(prwe.Start),
		exporterhelper.WithShutdown(prwe.Shutdown),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(prwCfg.ResourceToTelemetrySettings, exporter), nil
}

func createDefaultConfig() component.Config {
	return &Config{
		Namespace:       "",
		ExternalLabels:  map[string]string{},
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:             true,
			InitialInterval:     50 * time.Millisecond,
			MaxInterval:         200 * time.Millisecond,
			MaxElapsedTime:      1 * time.Minute,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://some.url:9411/api/prom/push",
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			ReadBufferSize:  0,
			WriteBufferSize: 512 * 1024,
			Timeout:         exporterhelper.NewDefaultTimeoutSettings().Timeout,
			Headers:         map[string]configopaque.String{},
		},
		// TODO(jbd): Adjust the default queue size.
		RemoteWriteQueue: RemoteWriteQueue{
			Enabled:      true,
			QueueSize:    10000,
			NumConsumers: 5,
		},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		ScopeInfo: &ScopeInfo{
			Enabled: false,
		},
		CreatedMetric: &CreatedMetric{
			Enabled: false,
		},
	}
}
