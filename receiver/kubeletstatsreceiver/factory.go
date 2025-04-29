// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

const (
	metricGroupsConfig               = "metric_groups"
	enableCPUUsageMetricsFeatureFlag = "receiver.kubeletstats.enableCPUUsageMetrics"
)

var enableCPUUsageMetrics = featuregate.GlobalRegistry().MustRegister(
	enableCPUUsageMetricsFeatureFlag,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled the container.cpu.utilization, k8s.pod.cpu.utilization and k8s.node.cpu.utilization metrics will be replaced by the container.cpu.usage, k8s.pod.cpu.usage and k8s.node.cpu.usage"),
	featuregate.WithRegisterFromVersion("v0.110.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27885"),
)

var defaultMetricGroups = []kubelet.MetricGroup{
	kubelet.ContainerMetricGroup,
	kubelet.PodMetricGroup,
	kubelet.NodeMetricGroup,
}

// NewFactory creates a factory for kubeletstats receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 10 * time.Second

	return &Config{
		ControllerConfig: scs,
		ClientConfig: kube.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeTLS,
			},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	rOptions, err := cfg.getReceiverOptions()
	if err != nil {
		return nil, err
	}
	rest, err := restClient(set.Logger, cfg)
	if err != nil {
		return nil, err
	}

	if enableCPUUsageMetrics.IsEnabled() {
		if cfg.Metrics.ContainerCPUUtilization.Enabled {
			cfg.Metrics.ContainerCPUUtilization.Enabled = false
			cfg.Metrics.ContainerCPUUsage.Enabled = true
		}
		if cfg.Metrics.K8sPodCPUUtilization.Enabled {
			cfg.Metrics.K8sPodCPUUtilization.Enabled = false
			cfg.Metrics.K8sPodCPUUsage.Enabled = true
		}
		if cfg.Metrics.K8sNodeCPUUtilization.Enabled {
			cfg.Metrics.K8sNodeCPUUtilization.Enabled = false
			cfg.Metrics.K8sNodeCPUUsage.Enabled = true
		}
	} else {
		if cfg.Metrics.ContainerCPUUtilization.Enabled {
			set.Logger.Warn("The default metric container.cpu.utilization is being replaced by the container.cpu.usage metric. Switch now by enabling the receiver.kubeletstats.enableCPUUsageMetrics feature gate.")
		}
		if cfg.Metrics.K8sPodCPUUtilization.Enabled {
			set.Logger.Warn("The default metric k8s.pod.cpu.utilization is being replaced by the k8s.pod.cpu.usage metric. Switch now by enabling the receiver.kubeletstats.enableCPUUsageMetrics feature gate.")
		}
		if cfg.Metrics.K8sNodeCPUUtilization.Enabled {
			set.Logger.Warn("The default metric k8s.node.cpu.utilization is being replaced by the k8s.node.cpu.usage metric. Switch now by enabling the receiver.kubeletstats.enableCPUUsageMetrics feature gate.")
		}
	}

	scrp, err := newKubeletScraper(rest, set, rOptions, cfg.MetricsBuilderConfig, cfg.NodeName)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, set, consumer, scraperhelper.AddScraper(metadata.Type, scrp))
}

func restClient(logger *zap.Logger, cfg *Config) (kubelet.RestClient, error) {
	clientProvider, err := kube.NewClientProvider(cfg.Endpoint, &cfg.ClientConfig, logger)
	if err != nil {
		return nil, err
	}
	client, err := clientProvider.BuildClient()
	if err != nil {
		return nil, err
	}
	rest := kubelet.NewRestClient(client)
	return rest, nil
}
