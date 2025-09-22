// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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
)

const (
	// Main session count SQL - keeping for backward compatibility
	sessionCountSQL = "SELECT COUNT(*) as SESSION_COUNT FROM v$session WHERE type = 'USER'"
)

type dbProviderFunc func() (*sql.DB, error)

type newRelicOracleScraper struct {
	// Multiple clients for different metrics
	sessionCountClient   dbClient
	tablespaceClient     dbClient
	cpuUsageClient       dbClient
	memoryClient         dbClient
	
	db                   *sql.DB
	clientProviderFunc   clientProviderFunc
	mb                   *metadata.MetricsBuilder
	dbProviderFunc       dbProviderFunc
	logger               *zap.Logger
	instanceName         string
	hostName             string
	scrapeCfg            scraperhelper.ControllerConfig
	startTime            pcommon.Timestamp
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

func newScraper(metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName, hostName string) (scraper.Metrics, error) {
	s := &newRelicOracleScraper{
		mb:                   metricsBuilder,
		metricsBuilderConfig: metricsBuilderConfig,
		scrapeCfg:            scrapeCfg,
		logger:               logger,
		dbProviderFunc:       providerFunc,
		clientProviderFunc:   clientProviderFunc,
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
	
	// Initialize the main session count client
	s.sessionCountClient = s.clientProviderFunc(s.db, sessionCountSQL, s.logger)
	
	// Initialize clients for different metric categories using their respective SQL queries
	// Note: These will be initialized with generic queries, specific queries are handled in individual scrapers
	s.tablespaceClient = s.clientProviderFunc(s.db, "SELECT 1 FROM DUAL", s.logger) // Placeholder
	s.cpuUsageClient = s.clientProviderFunc(s.db, "SELECT 1 FROM DUAL", s.logger)   // Placeholder
	s.memoryClient = s.clientProviderFunc(s.db, "SELECT 1 FROM DUAL", s.logger)     // Placeholder
	
	return nil
}

func (s *newRelicOracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin New Relic Oracle scrape")

	var scrapeErrors []error

	// Call individual scrape functions organized by category
	
	// Session-related metrics
	scrapeErrors = append(scrapeErrors, s.scrapeSessionCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeActiveSessionCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeInactiveSessionCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeBlockedSessionCount(ctx)...)
	
	// Tablespace-related metrics
	scrapeErrors = append(scrapeErrors, s.scrapeTablespaceMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeTablespaceUsageMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeTempTablespaceMetrics(ctx)...)
	
	// Performance-related metrics
	scrapeErrors = append(scrapeErrors, s.scrapeCpuUsage(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeMemoryMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeWaitEvents(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeProcessCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeRedoLogSwitches(ctx)...)
	
	// Database-related metrics
	scrapeErrors = append(scrapeErrors, s.scrapeDatabaseSize(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeUserCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeLockCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeArchiveLogCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.scrapeInvalidObjectsCount(ctx)...)

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
