// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/scrapers"
)

const (
	// keepalive for connection 
	keepAlive = 30 * time.Second
)

type dbProviderFunc func() (*sql.DB, error)

type newRelicOracleScraper struct {
	// Only keep session scraper for simplicity
	sessionScraper       *scrapers.SessionScraper
	
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	dbProviderFunc       dbProviderFunc
	logger               *zap.Logger
	instanceName         string
	hostName             string
	scrapeCfg            scraperhelper.ControllerConfig
	startTime            pcommon.Timestamp
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

func newScraper(metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, instanceName, hostName string) (scraper.Metrics, error) {
	s := &newRelicOracleScraper{
		mb:                   metricsBuilder,
		dbProviderFunc:       providerFunc,
		logger:               logger,
		scrapeCfg:            scrapeCfg,
		metricsBuilderConfig: metricsBuilderConfig,
		instanceName:         instanceName,
		hostName:             hostName,
	}
	return scraper.NewMetrics(s.scrape, scraper.WithShutdown(s.shutdown), scraper.WithStart(s.start))
}

func (s *newRelicOracleScraper) start(context.Context, component.Host) error {
	s.startTime = pcommon.NewTimestampFromTime(time.Now())
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	
	// Initialize session scraper with direct DB connection
	s.sessionScraper = scrapers.NewSessionScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)
	
	return nil
}

func (s *newRelicOracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin New Relic Oracle scrape")

	var scrapeErrors []error

	// Only scrape session count metric - keeping it simple
	scrapeErrors = append(scrapeErrors, s.sessionScraper.ScrapeSessionCount(ctx)...)

	// Build the resource with instance and host information
	rb := s.mb.NewResourceBuilder()
	rb.SetNewrelicoracledbInstanceName(s.instanceName)
	rb.SetHostName(s.hostName)
	out := s.mb.Emit(metadata.WithResource(rb.Emit()))
	
	s.logger.Debug("Done New Relic Oracle scraping", zap.Int("total_errors", len(scrapeErrors)))
	if len(scrapeErrors) > 0 {
		return out, scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return out, nil
}

func (s *newRelicOracleScraper) shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}