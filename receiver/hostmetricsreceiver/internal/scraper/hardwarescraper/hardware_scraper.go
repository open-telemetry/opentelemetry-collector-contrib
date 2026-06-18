// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hardwarescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

var ErrHwmonUnavailable = errors.New("hwmon not available")

const (
	hardwareTemperatureMetricsLen = 2
	metricsLen                    = hardwareTemperatureMetricsLen
)

// temperatureScraper interface for temperature sensor scraping
type temperatureScraper interface {
	start(context.Context) error
	scrape(context.Context, *metadata.MetricsBuilder) error
}

type hardwareScraper struct {
	logger             *zap.Logger
	mb                 *metadata.MetricsBuilder
	config             *Config
	temperatureScraper temperatureScraper
}

// newHardwareScraper creates a new hardware metrics scraper
func newHardwareScraper(_ context.Context, settings scraper.Settings, cfg *Config) *hardwareScraper {
	mb := metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

	var tempScraper temperatureScraper
	if cfg.Temperature != nil {
		tempScraper = &hardwareTemperatureScraper{
			logger:               settings.Logger,
			config:               cfg.Temperature,
			hwmonPath:            cfg.HwmonPath,
			metricsBuilderConfig: cfg.MetricsBuilderConfig,
		}
	}

	return &hardwareScraper{
		logger:             settings.Logger,
		mb:                 mb,
		config:             cfg,
		temperatureScraper: tempScraper,
	}
}

func (s *hardwareScraper) start(ctx context.Context, _ component.Host) error {
	if s.temperatureScraper != nil {
		if err := s.temperatureScraper.start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *hardwareScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors

	if s.temperatureScraper != nil {
		if err := s.temperatureScraper.scrape(ctx, s.mb); err != nil {
			s.logger.Debug("Temperature scraper returned error", zap.Error(err))
			errs.AddPartial(metricsLen, err)
		}
	}

	// Future scrapers can be added here:
	// if s.fanScraper != nil { ... }
	// if s.voltageScraper != nil { ... }

	return s.mb.Emit(), errs.Combine()
}
