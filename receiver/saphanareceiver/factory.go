// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

const (
	defaultEndpoint = "localhost:33015"
)

// NewFactory creates a factory for SAP HANA receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: defaultEndpoint,
		},
		TLSClientSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		ScraperControllerSettings: scs,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}

var errConfigNotSAPHANA = errors.New("config was not an sap hana receiver config")

func createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotSAPHANA
	}
	scraper, err := newSapHanaScraper(set, c, &defaultConnectionFactory{})
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&c.ScraperControllerSettings, set, consumer, scraperhelper.AddScraper(scraper))
}
