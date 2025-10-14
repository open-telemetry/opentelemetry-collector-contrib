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
	sessionScraper     *scrapers.SessionScraper
	tablespaceScraper  *scrapers.TablespaceScraper
	coreScraper        *scrapers.CoreScraper
	pdbScraper         *scrapers.PdbScraper
	systemScraper      *scrapers.SystemScraper
	slowQueriesScraper *scrapers.SlowQueriesScraper
	blockingScraper    *scrapers.BlockingScraper
	waitEventsScraper  *scrapers.WaitEventsScraper
	individualQueriesScraper *scrapers.IndividualQueriesScraper
	connectionScraper *scrapers.ConnectionScraper
	containerScraper  *scrapers.ContainerScraper
	racScraper        *scrapers.RacScraper

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

	// Initialize slow queries scraper with direct DB connection
	s.slowQueriesScraper = scrapers.NewSlowQueriesScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize individual queries scraper with direct DB connection
	s.individualQueriesScraper = scrapers.NewIndividualQueriesScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)
	// Initialize blocking scraper with direct DB connection
	s.blockingScraper = scrapers.NewBlockingScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)
	// Initialize wait events scraper with direct DB connection
	s.waitEventsScraper = scrapers.NewWaitEventsScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)
	// Initialize connection scraper with direct DB connection
	s.connectionScraper = scrapers.NewConnectionScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize container scraper with direct DB connection
	s.containerScraper = scrapers.NewContainerScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	// Initialize RAC scraper with direct DB connection
	s.racScraper = scrapers.NewRacScraper(s.db, s.mb, s.logger, s.instanceName, s.metricsBuilderConfig)

	return nil
}

func (s *newRelicOracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin New Relic Oracle scrape")

	// Create context with timeout for the entire scrape operation
	// IMPORTANT: Use the original context, not a child context to ensure proper cancellation
	scrapeCtx := ctx
	if s.scrapeCfg.Timeout > 0 {
		var cancel context.CancelFunc
		scrapeCtx, cancel = context.WithTimeout(ctx, s.scrapeCfg.Timeout)
		defer cancel()
	}

	// Channel to collect errors from concurrent scrapers
	const maxErrors = 100 // Buffer size to prevent blocking
	errChan := make(chan error, maxErrors)

	// WaitGroup to coordinate concurrent scrapers
	var wg sync.WaitGroup

	// First execute slow queries scraper to get query IDs
	s.logger.Debug("Starting slow queries scraper to get query IDs")
	queryIDs, slowQueryErrs := s.slowQueriesScraper.ScrapeSlowQueries(scrapeCtx)

	s.logger.Info("Slow queries scraper completed",
		zap.Int("query_ids_found", len(queryIDs)),
		zap.Strings("query_ids", queryIDs),
		zap.Int("slow_query_errors", len(slowQueryErrs)))

	// Add slow query errors to our error collection
	for _, err := range slowQueryErrs {
		select {
		case errChan <- err:
		default:
			s.logger.Warn("Error channel full, dropping slow query error", zap.Error(err))
		}
	}

	// Define scraper functions that don't depend on slow queries
	independentScraperFuncs := []ScraperFunc{
		s.sessionScraper.ScrapeSessionCount,
		s.tablespaceScraper.ScrapeTablespaceMetrics,
		s.coreScraper.ScrapeCoreMetrics,
		s.pdbScraper.ScrapePdbMetrics,
		s.systemScraper.ScrapeSystemMetrics,
		s.blockingScraper.ScrapeBlockingQueries,
		s.waitEventsScraper.ScrapeWaitEvents,
		s.connectionScraper.ScrapeConnectionMetrics,
		s.containerScraper.ScrapeContainerMetrics,
		s.racScraper.ScrapeRacMetrics,
	}

	// Launch concurrent scrapers for independent scrapers
	for i, scraperFunc := range independentScraperFuncs {
		wg.Add(1)
		go func(index int, fn ScraperFunc) {
			defer wg.Done()

			// Check for context cancellation before starting
			select {
			case <-scrapeCtx.Done():
				s.logger.Debug("Context cancelled before starting scraper",
					zap.Int("scraper_index", index),
					zap.Error(scrapeCtx.Err()))
				return
			default:
			}

			s.logger.Debug("Starting scraper", zap.Int("scraper_index", index))
			startTime := time.Now()

			// Execute the scraper function with cancellation check
			errs := fn(scrapeCtx)

			duration := time.Since(startTime)
			s.logger.Debug("Completed scraper",
				zap.Int("scraper_index", index),
				zap.Duration("duration", duration),
				zap.Int("error_count", len(errs)))

			// Send errors to the error channel with cancellation check
			for _, err := range errs {
				select {
				case errChan <- err:
				case <-scrapeCtx.Done():
					// Context cancelled, stop sending errors and exit goroutine
					s.logger.Debug("Context cancelled while sending errors",
						zap.Int("scraper_index", index),
						zap.Error(scrapeCtx.Err()))
					return
				default:
					// Channel is full, log and continue
					s.logger.Warn("Error channel full, dropping error",
						zap.Error(err),
						zap.Int("scraper_index", index))
				}
			}
		}(i, scraperFunc)
	}

	// Execute individual queries scraper with filtered query IDs
	wg.Add(1)
	go func() {
		defer wg.Done()

		s.logger.Debug("Starting individual queries scraper with filtered IDs", zap.Int("query_ids_count", len(queryIDs)))
		s.logger.Info("Individual queries scraper about to start",
			zap.Int("query_ids_available", len(queryIDs)),
			zap.Strings("available_query_ids", queryIDs))
		startTime := time.Now()

		// Execute the individual queries scraper with query ID filter
		errs := s.individualQueriesScraper.ScrapeIndividualQueries(scrapeCtx, queryIDs)

		duration := time.Since(startTime)
		s.logger.Debug("Completed individual queries scraper",
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
				s.logger.Warn("Error channel full, dropping individual query error", zap.Error(err))
			}
		}
	}()

	// Close error channel when all scrapers are done
	// Close error channel when all scrapers are done, with timeout protection
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
		close(errChan)
	}()

	// Collect all errors from the channel with cancellation handling
	var scrapeErrors []error
	collecting := true
	for collecting {
		select {
		case err, ok := <-errChan:
			if !ok {
				collecting = false
				break
			}
			scrapeErrors = append(scrapeErrors, err)
		case <-scrapeCtx.Done():
			// Context cancelled, stop collecting errors
			s.logger.Warn("Context cancelled while collecting errors", zap.Error(scrapeCtx.Err()))
			scrapeErrors = append(scrapeErrors, fmt.Errorf("scrape operation cancelled: %w", scrapeCtx.Err()))
			collecting = false
		case <-done:
			// All scrapers completed, collect remaining errors
			for err := range errChan {
				scrapeErrors = append(scrapeErrors, err)
			}
			collecting = false
		}
	}

	// Build the resource with instance and host information
	rb := s.mb.NewResourceBuilder()
	rb.SetNewrelicoracledbInstanceName(s.instanceName)
	rb.SetHostName(s.hostName)
	out := s.mb.Emit(metadata.WithResource(rb.Emit()))

	s.logger.Debug("Done New Relic Oracle scraping",
		zap.Int("total_errors", len(scrapeErrors)),
		zap.Int("independent_scrapers_count", len(independentScraperFuncs)),
		zap.Int("total_scrapers_count", len(independentScraperFuncs)+2)) // +2 for slow queries and individual queries

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
