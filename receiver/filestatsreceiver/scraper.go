// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"context"
	"os"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

type fsScraper struct {
	include string
	logger  *zap.Logger
	mb      *metadata.MetricsBuilder
}

func (s *fsScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	matches, err := doublestar.FilepathGlob(s.include)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	var scrapeErrors []error

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, match := range matches {
		fileinfo, err := os.Stat(match)
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
			continue
		}
		s.mb.RecordFileSizeDataPoint(now, fileinfo.Size())
		s.mb.RecordFileMtimeDataPoint(now, fileinfo.ModTime().Unix())
		collectStats(now, fileinfo, s.mb, s.logger)

		rb := s.mb.NewResourceBuilder()
		rb.SetFileName(fileinfo.Name())
		rb.SetFilePath(match)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	s.mb.RecordFileCountDataPoint(now, int64(len(matches)))
	s.mb.EmitForResource()

	if len(scrapeErrors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *fsScraper {
	return &fsScraper{
		include: cfg.Include,
		logger:  settings.Logger,
		mb:      metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
