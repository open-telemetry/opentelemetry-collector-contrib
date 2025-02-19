// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // init() of this package registers service discovery impl.
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

// This file implements config for Prometheus receiver.
var useCreatedMetricGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.prometheusreceiver.UseCreatedMetric",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Prometheus receiver will"+
		" retrieve the start time for Summary, Histogram and Sum metrics from _created metric"),
)

var enableNativeHistogramsGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.prometheusreceiver.EnableNativeHistograms",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Prometheus receiver will convert"+
		" Prometheus native histograms to OTEL exponential histograms and ignore"+
		" those Prometheus classic histograms that have a native histogram alternative"),
)

// NewFactory creates a new Prometheus receiver factory.
func NewFactory() receiver.Factory {
	// Since Prometheus 3.0, the default validation scheme for metric names is UTF8.
	// This includes ScrapeManager lib that is used by the Promethes receiver.
	// We need to set the validation scheme to _something_ to avoid panics, and
	// UTF8 is the default in Prometheus.
	model.NameValidationScheme = model.UTF8Validation

	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		PrometheusConfig: &PromConfig{
			GlobalConfig: promconfig.DefaultGlobalConfig,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	configWarnings(set.Logger, cfg.(*Config))
	addDefaultFallbackScrapeProtocol(cfg.(*Config))
	return newPrometheusReceiver(set, cfg.(*Config), nextConsumer), nil
}

func configWarnings(logger *zap.Logger, cfg *Config) {
	for _, sc := range cfg.PrometheusConfig.ScrapeConfigs {
		for _, rc := range sc.MetricRelabelConfigs {
			if rc.TargetLabel == "__name__" {
				logger.Warn("metric renaming using metric_relabel_configs will result in unknown-typed metrics without a unit or description", zap.String("job", sc.JobName))
			}
		}
	}
}

func addDefaultFallbackScrapeProtocol(cfg *Config) {
	for _, sc := range cfg.PrometheusConfig.ScrapeConfigs {
		if sc.ScrapeFallbackProtocol == "" {
			sc.ScrapeFallbackProtocol = promconfig.PrometheusText1_0_0
		}
	}
}
