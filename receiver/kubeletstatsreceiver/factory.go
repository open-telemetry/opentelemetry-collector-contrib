// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"context"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

const (
	metricGroupsConfig = "metric_groups"
)

var defaultMetricGroups = []kubelet.MetricGroup{
	kubelet.ContainerMetricGroup,
	kubelet.PodMetricGroup,
	kubelet.NodeMetricGroup,
}

// NewFactory creates a factory for kubeletstats receiver.
func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType))
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
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
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

	scrp, err := newKubeletScraper(rest, set, rOptions, cfg.MetricsBuilderConfig, cfg.NodeName)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, set, consumer, scraperhelper.AddMetricsScraper(metadata.Type, scrp))
}

func restClient(logger *zap.Logger, cfg *Config) (kubelet.RestClient, error) {
	// Create a transport with connection pooling settings configured.
	// This transport will be used as the base for the kubelet client,
	// allowing connections to be reused across scrapes to reduce TLS handshake overhead.
	transport := createTransport(cfg)

	clientProvider, err := kube.NewClientProviderWithRoundTripper(cfg.Endpoint, &cfg.ClientConfig, logger, transport)
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

// createTransport creates an http.Transport with connection pooling settings from the config.
// These settings help reduce TLS handshake overhead by reusing connections between scrapes.
func createTransport(cfg *Config) *http.Transport {
	// Clone the default transport to preserve reasonable defaults
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Apply dialer timeout if configured (backwards compat with confignet.TCPAddrConfig).
	if cfg.DialerConfig.Timeout > 0 {
		d := &net.Dialer{Timeout: cfg.DialerConfig.Timeout}
		transport.DialContext = d.DialContext
	}

	// Apply connection pooling settings from config
	if cfg.MaxIdleConns > 0 {
		transport.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}
	if cfg.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = cfg.IdleConnTimeout
	}

	return transport
}
