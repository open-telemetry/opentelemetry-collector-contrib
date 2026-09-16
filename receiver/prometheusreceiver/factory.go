// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"

	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/aws"
	_ "github.com/prometheus/prometheus/discovery/azure"
	_ "github.com/prometheus/prometheus/discovery/consul"
	_ "github.com/prometheus/prometheus/discovery/digitalocean"
	_ "github.com/prometheus/prometheus/discovery/dns"
	_ "github.com/prometheus/prometheus/discovery/eureka"
	_ "github.com/prometheus/prometheus/discovery/file"
	_ "github.com/prometheus/prometheus/discovery/gce"
	_ "github.com/prometheus/prometheus/discovery/hetzner"
	_ "github.com/prometheus/prometheus/discovery/http"
	_ "github.com/prometheus/prometheus/discovery/ionos"
	_ "github.com/prometheus/prometheus/discovery/kubernetes"
	_ "github.com/prometheus/prometheus/discovery/linode"
	_ "github.com/prometheus/prometheus/discovery/marathon"
	_ "github.com/prometheus/prometheus/discovery/moby"
	_ "github.com/prometheus/prometheus/discovery/nomad"
	_ "github.com/prometheus/prometheus/discovery/oci"
	_ "github.com/prometheus/prometheus/discovery/openstack"
	_ "github.com/prometheus/prometheus/discovery/ovhcloud"
	_ "github.com/prometheus/prometheus/discovery/puppetdb"
	_ "github.com/prometheus/prometheus/discovery/scaleway"
	_ "github.com/prometheus/prometheus/discovery/stackit"
	_ "github.com/prometheus/prometheus/discovery/triton"
	_ "github.com/prometheus/prometheus/discovery/uyuni"
	_ "github.com/prometheus/prometheus/discovery/vultr"
	_ "github.com/prometheus/prometheus/discovery/xds"
	_ "github.com/prometheus/prometheus/discovery/zookeeper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

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
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	taClientConfig.MaxIdleConns = 0    //nolint:staticcheck // SA1019: see TODO above
	taClientConfig.IdleConnTimeout = 0 //nolint:staticcheck // SA1019: see TODO above
	taClientConfig.ForceAttemptHTTP2 = false
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
	configWarnings(set.Logger, cfg.(*Config))
	return newPrometheusReceiver(set, cfg.(*Config), nextConsumer)
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
