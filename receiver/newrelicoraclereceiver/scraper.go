// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
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

// ScraperFunc represents a function that scrapes metrics and returns errors
type ScraperFunc func(context.Context) []error

type dbProviderFunc func() (*sql.DB, error)

type newRelicOracleScraper struct {
	// Keep session scraper and add tablespace scraper and core scraper
	sessionScraper    *scrapers.SessionScraper
	tablespaceScraper *scrapers.TablespaceScraper
	coreScraper       *scrapers.CoreScraper
	pdbScraper        *scrapers.PdbScraper
	systemScraper     *scrapers.SystemScraper
	connectionScraper *scrapers.ConnectionScraper

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

	// Initialize tablespace scraper with direct DB connection
	s.tablespaceScraper = scrapers.NewTablespaceScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize core scraper with direct DB connection
	s.coreScraper = scrapers.NewCoreScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize PDB scraper with direct DB connection
	s.pdbScraper = scrapers.NewPdbScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize system scraper with direct DB connection
	s.systemScraper = scrapers.NewSystemScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize connection scraper with direct DB connection
	s.connectionScraper = scrapers.NewConnectionScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	return nil
}

func (s *newRelicOracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin New Relic Oracle scrape")

	// Create context with timeout for the entire scrape operation
	scrapeCtx, cancel := context.WithTimeout(ctx, s.scrapeCfg.Timeout)
	defer cancel()

	// Channel to collect errors from concurrent scrapers
	const maxErrors = 100 // Buffer size to prevent blocking
	errChan := make(chan error, maxErrors)

	// WaitGroup to coordinate concurrent scrapers
	var wg sync.WaitGroup

	// Define all scraper functions
	scraperFuncs := []ScraperFunc{
		s.sessionScraper.ScrapeSessionCount,
		s.tablespaceScraper.ScrapeTablespaceMetrics,
		s.coreScraper.ScrapeCoreMetrics,
		s.pdbScraper.ScrapePdbMetrics,
		s.systemScraper.ScrapeSystemMetrics,
		s.connectionScraper.ScrapeConnectionMetrics,
	}

	// Launch concurrent scrapers
	for i, scraperFunc := range scraperFuncs {
		wg.Add(1)
		go func(index int, fn ScraperFunc) {
			defer wg.Done()

			s.logger.Debug("Starting scraper", zap.Int("scraper_index", index))
			startTime := time.Now()

			// Execute the scraper function
			errs := fn(scrapeCtx)

			duration := time.Since(startTime)
			s.logger.Debug("Completed scraper",
				zap.Int("scraper_index", index),
				zap.Duration("duration", duration),
				zap.Int("error_count", len(errs)))

			// Send errors to the error channel
			for _, err := range errs {
				select {
				case errChan <- err:
				case <-scrapeCtx.Done():
					// Context cancelled, stop sending errors
					return
				default:
					// Channel is full, log and continue
					s.logger.Warn("Error channel full, dropping error", zap.Error(err))
				}
			}
		}(i, scraperFunc)
	}

	// Close error channel when all scrapers are done
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect all errors from the channel
	var scrapeErrors []error
	for err := range errChan {
		scrapeErrors = append(scrapeErrors, err)
	}

	// Check if context was cancelled
	if scrapeCtx.Err() != nil {
		scrapeErrors = append(scrapeErrors, fmt.Errorf("scrape operation timed out or was cancelled: %w", scrapeCtx.Err()))
	}

	// Build the resource with instance and host information
	rb := s.mb.NewResourceBuilder()
	rb.SetNewrelicoracledbInstanceName(s.instanceName)
	rb.SetHostName(s.hostName)
	out := s.mb.Emit(metadata.WithResource(rb.Emit()))

	s.logger.Debug("Done New Relic Oracle scraping",
		zap.Int("total_errors", len(scrapeErrors)),
		zap.Int("scrapers_count", len(scraperFuncs)))

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
