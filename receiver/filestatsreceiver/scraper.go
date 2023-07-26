// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

type scraper struct {
	include string
	logger  *zap.Logger
	rb      *metadata.ResourceBuilder
	mb      *metadata.MetricsBuilder
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
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
		path, err := filepath.Abs(fileinfo.Name())
		if err != nil {
			scrapeErrors = append(scrapeErrors, err)
			continue
		}
		s.mb.RecordFileSizeDataPoint(now, fileinfo.Size())
		s.mb.RecordFileMtimeDataPoint(now, fileinfo.ModTime().Unix())
		collectStats(now, fileinfo, s.mb, s.logger)

		s.rb.SetFileName(fileinfo.Name())
		s.rb.SetFilePath(path)
		s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
	}

	if len(scrapeErrors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.CreateSettings) *scraper {
	return &scraper{
		include: cfg.Include,
		logger:  settings.TelemetrySettings.Logger,
		rb:      metadata.NewResourceBuilder(cfg.ResourceAttributes),
		mb:      metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}
