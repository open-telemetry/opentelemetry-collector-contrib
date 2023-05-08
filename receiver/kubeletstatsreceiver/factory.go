// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

const (
	typeStr            = "kubeletstats"
	stability          = component.StabilityLevelBeta
	metricGroupsConfig = "metric_groups"
)

var defaultMetricGroups = []kubelet.MetricGroup{
	kubelet.ContainerMetricGroup,
	kubelet.PodMetricGroup,
	kubelet.NodeMetricGroup,
}

// NewFactory creates a factory for kubeletstats receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultScraperControllerSettings(typeStr)
	scs.CollectionInterval = 10 * time.Second

	return &Config{
		ScraperControllerSettings: scs,
		ClientConfig: kube.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeTLS,
			},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
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

	scrp, err := newKubletScraper(rest, set, rOptions, cfg.MetricsBuilderConfig)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, set, consumer, scraperhelper.AddScraper(scrp))
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
