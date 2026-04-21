// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/xscraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"
)

func newScraper(cfg *Config, settings receiver.Settings) (xscraper.Profiles, error) {
	if cfg.Include != "" {
		fs := internal.FileScraper{
			Logger:  settings.Logger,
			Include: cfg.Include,
		}
		return xscraper.NewProfiles(fs.Scrape)
	}
	if cfg.Endpoint != "" {
		httpScraper := &internal.HTTPClientScraper{
			ClientConfig: cfg.ClientConfig,
			Settings:     settings.TelemetrySettings,
		}
		return httpScraper, nil
	}

	return &internal.SelfScraper{
		BlockProfileFraction: cfg.BlockProfileFraction,
		MutexProfileFraction: cfg.MutexProfileFraction,
	}, nil
}
