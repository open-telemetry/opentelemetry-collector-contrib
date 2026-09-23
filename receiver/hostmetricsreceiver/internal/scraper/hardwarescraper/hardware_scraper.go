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

type hardwareScraper struct {
	logger             *zap.Logger
	mb                 *metadata.MetricsBuilder
	config             *Config
	temperatureScraper *hardwareTemperatureScraper
}

// newHardwareScraper creates a new hardware metrics scraper
func newHardwareScraper(_ context.Context, settings scraper.Settings, cfg *Config) *hardwareScraper {
	mb := metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

	var tempScraper *hardwareTemperatureScraper
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
			// Preserve the sub-scraper's failure count instead of replacing it with
			// a constant in the wrapper.
			var partialErr scrapererror.PartialScrapeError
			if errors.As(err, &partialErr) {
				errs.AddPartial(partialErr.Failed, err)
			} else {
				errs.Add(err)
			}
		}
	}

	return s.mb.Emit(), errs.Combine()
}
