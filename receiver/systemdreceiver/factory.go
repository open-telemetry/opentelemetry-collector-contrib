// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"errors"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

var (
	errConfigNotValid = errors.New("config not valid for the systemd receiver")
	errNonLinux       = errors.New("systemd receiver is only supported on Linux")
)

// NewFactory creates a factory for systemd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 60 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Scope:                "system",
		Units:                []string{"*.service"},
	}
}

func createMetricsReceiver(_ context.Context, params receiver.Settings, rConf component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	if runtime.GOOS != "linux" {
		return nil, errNonLinux
	}

	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotValid
	}

	systemdScraper := newScraper(cfg, params)
	s, err := scraper.NewMetrics(systemdScraper.scrape, scraper.WithStart(systemdScraper.start), scraper.WithShutdown(systemdScraper.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, params, consumer, scraperhelper.AddScraper(metadata.Type, s))
}
