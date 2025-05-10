// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

const (
	defaultPath     = "/debug/vars"
	defaultEndpoint = "http://localhost:8000" + defaultPath
	defaultTimeout  = 3 * time.Second
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithMetrics(newMetricsReceiver, metadata.MetricsStability))
}

func newMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	rCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rCfg.(*Config)

	expVar := newExpVarScraper(cfg, set)
	scraper, err := scraperhelper.NewScraper(
		metadata.Type,
		expVar.scrape,
		scraperhelper.WithStart(expVar.start),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig,
		set,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}

func newDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = defaultTimeout
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		ClientConfig:         clientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}
