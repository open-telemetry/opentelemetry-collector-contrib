// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
)

type pprofScraper struct {
	logger   *zap.Logger
	config   *Config
	settings component.TelemetrySettings
}

func newScraper(cfg *Config, settings receiver.Settings) *pprofScraper {
	return &pprofScraper{
		logger:   settings.Logger,
		settings: settings.TelemetrySettings,
		config:   cfg,
	}
}

func (*pprofScraper) start(_ context.Context, _ component.Host) error {
	return nil
}

func (s *pprofScraper) scrape(_ context.Context) (pprofile.Profiles, error) {
	matches, err := doublestar.FilepathGlob(s.config.Include)
	if err != nil {
		return pprofile.NewProfiles(), err
	}

	var scrapeErrors []error
	result := pprofile.NewProfiles()

	for _, match := range matches {
		reader, err := os.Open(match)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to open file %s: %w", match, err))
			continue
		}

		pprofProfile, err := profile.Parse(reader)
		reader.Close()
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse pprof data from %s: %w", match, err))
			continue
		}

		profiles, err := pprof.ConvertPprofToProfiles(pprofProfile)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to convert pprof to profiles from %s: %w", match, err))
			continue
		}

		profiles.ResourceProfiles().MoveAndAppendTo(result.ResourceProfiles())
		s.logger.Debug("Successfully scraped pprof file", zap.String("file", match))
	}

	if len(scrapeErrors) > 0 {
		return result, scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return result, nil
}
