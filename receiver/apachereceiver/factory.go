// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

import (
	"context"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

const (
	httpDefaultPort  = "80"
	httpsDefaultPort = "443"
)

// NewFactory creates a factory for apache receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = 10 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		ClientConfig:         clientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func parseResourceAttributes(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", err
	}

	serverName := u.Hostname()
	port := u.Port()

	if port == "" {
		switch u.Scheme {
		case "https":
			port = httpsDefaultPort
		case "http":
			port = httpDefaultPort
		}
		// else: unknown scheme, leave port as empty string
	}

	return serverName, port, err
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	serverName, port, err := parseResourceAttributes(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	ns := newApacheScraper(params, cfg, serverName, port)
	s, err := scraper.NewMetrics(ns.scrape, scraper.WithStart(ns.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
