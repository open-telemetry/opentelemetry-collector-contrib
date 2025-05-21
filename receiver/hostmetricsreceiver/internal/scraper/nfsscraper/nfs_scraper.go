// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
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

// newNfsScraper creates an Uptime related metric
func newNfsScraper(settings scraper.Settings, cfg *Config) (*nfsScraper, error) {
	return &nfsScraper{
		settings:  settings,
		config:    cfg,
		nfsStats:  getNfsStats,
		nfsdStats: getNfsdStats,
	}, nil
}

func (s *nfsScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *nfsScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors

	NfsStats, err := s.nfsStats()
	if err != nil {
		errs.AddPartial(nfsMetricsLen, err)
	}

	NfsdStats, err := s.nfsdStats()
	if err != nil {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	nothing(NfsStats, NfsdStats)

	return s.mb.Emit(), errs.Combine()
}
