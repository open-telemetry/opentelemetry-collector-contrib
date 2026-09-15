// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"context"
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

type FileScraper struct {
	Include   string
	Logger    *zap.Logger
	BuildInfo component.BuildInfo
}

func (fs FileScraper) Scrape(_ context.Context) (pprofile.Profiles, error) {
	matches, err := doublestar.FilepathGlob(fs.Include)
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

		// set ScopeName and Version
		for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
			rp := profiles.ResourceProfiles().At(i)
			for j := 0; j < rp.ScopeProfiles().Len(); j++ {
				sp := rp.ScopeProfiles().At(j)
				sp.Scope().SetName(metadata.ScopeName + "/filescraper")
				sp.Scope().SetVersion(fs.BuildInfo.Version)
			}
		}

		if err := profiles.MergeTo(result); err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to merge profiles from %s: %w", match, err))
			continue
		}
		fs.Logger.Debug("Successfully scraped pprof file", zap.String("file", match))
	}

	if len(scrapeErrors) > 0 {
		return result, scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return result, nil
}
