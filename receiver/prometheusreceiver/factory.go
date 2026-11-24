// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"

	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/plugins" // init() of this package registers service discovery impl.
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
)

// NewFactory creates a new Prometheus receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	taClientConfig := confighttp.NewDefaultClientConfig()
	return &Config{
		PrometheusConfig: &PromConfig{
			GlobalConfig: promconfig.DefaultGlobalConfig,
		},
		TargetAllocator: configoptional.Default(targetallocator.Config{
			ClientConfig: taClientConfig,
		}),
		APIServer: apiserver.DefaultConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	pCfg := cfg.(*Config)
	rcvr, err := newPrometheusReceiver(set, pCfg, nextConsumer)
	if err != nil {
		return nil, err
	}
	configWarnings(set.Logger, pCfg)
	return rcvr, nil
}

func configWarnings(logger *zap.Logger, cfg *Config) {
	if cfg.PrometheusConfig == nil {
		return
	}
	globalProtocols := cfg.PrometheusConfig.GlobalConfig.ScrapeProtocols
	globalHasOM2 := internal.WarnOpenMetrics2GlobalConfig(logger, globalProtocols)
	scrapeConfigs := cfg.PrometheusConfig.ScrapeConfigs
	if len(cfg.PrometheusConfig.ScrapeConfigFiles) > 0 {
		if loaded, err := (*promconfig.Config)(cfg.PrometheusConfig).GetScrapeConfigs(); err == nil {
			scrapeConfigs = loaded
		}
	}
	for _, sc := range scrapeConfigs {
		internal.WarnOpenMetrics2ScrapeConfig(logger, sc, globalHasOM2, globalProtocols)
		for _, rc := range sc.MetricRelabelConfigs {
			if rc.TargetLabel == "__name__" {
				logger.Warn("metric renaming using metric_relabel_configs will result in unknown-typed metrics without a unit or description", zap.String("job", sc.JobName))
			}
		}
	}
}
