// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

// NewFactory returns the receiver factory for the vcenterreceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 2 * time.Minute

	return &Config{
		ControllerConfig:     cfg,
		ClientConfig:         configtls.ClientConfig{},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

var errConfigNotVcenter = errors.New("config was not an vcenter receiver config")

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotVcenter
	}
	vr := newVmwareVcenterScraper(params.Logger, cfg, params)

	scraper, err := scraperhelper.NewScraperWithoutType(
		vr.scrape,
		scraperhelper.WithStart(vr.Start),
		scraperhelper.WithShutdown(vr.Shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig,
		params,
		consumer,
		scraperhelper.AddScraperWithType(metadata.Type, scraper),
	)
}
