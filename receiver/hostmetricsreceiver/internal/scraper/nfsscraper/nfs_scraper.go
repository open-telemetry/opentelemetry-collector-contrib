// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

const (
	nfsMetricsLen  = 97
	nfsdMetricsLen = 113
)

// nfsScraper for NFS Metrics
type nfsScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	nfsStats  func() (*NfsStats, error)
	nfsdStats func() (*NfsdStats, error)
}

// newNfsScraper creates an NFS Scraper related metric
func newNfsScraper(settings scraper.Settings, cfg *Config) *nfsScraper {
	return &nfsScraper{
		settings:  settings,
		config:    cfg,
		nfsStats:  getNfsStats,
		nfsdStats: getNfsdStats,
	}
}

func (s *nfsScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *nfsScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors

	nfsStats, err := s.nfsStats()
	if err != nil {
		errs.AddPartial(nfsMetricsLen, err)
	}

	nfsdStats, err := s.nfsdStats()
	if err != nil {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	err = s.addMetrics(&nfsStats, &nfsdStats)
	if err != nil {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *scraper) addMetrics(nfsStats *NfsStats, nfsdStats *NfsdStats) error {
{
	now := pcommon.NewTimestampFromTime(time.Now())

	
}
