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
	// Keep session scraper and add tablespace scraper and core scraper
	sessionScraper         *scrapers.SessionScraper
	tablespaceScraper      *scrapers.TablespaceScraper
	coreScraper            *scrapers.CoreScraper
	pdbScraper             *scrapers.PdbScraper
	systemScraper          *scrapers.SystemScraper
	individualQueryScraper *scrapers.IndividualQueryScraper
	config                 *Config

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

func newScraper(metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, instanceName, hostName string, config *Config) (scraper.Metrics, error) {
	s := &newRelicOracleScraper{
		mb:                   metricsBuilder,
		dbProviderFunc:       providerFunc,
		logger:               logger,
		scrapeCfg:            scrapeCfg,
		metricsBuilderConfig: metricsBuilderConfig,
		instanceName:         instanceName,
		hostName:             hostName,
		config:               config,
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

	// Initialize individual query scraper
	s.individualQueryScraper = scrapers.NewIndividualQueryScraper(s.logger)

	return nil
}

func (s *newRelicOracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin New Relic Oracle scrape")

	var scrapeErrors []error

	// Scrape session count, tablespace metrics, core metrics, PDB metrics, and system metrics
	scrapeErrors = append(scrapeErrors, s.sessionScraper.ScrapeSessionCount(ctx)...)
	scrapeErrors = append(scrapeErrors, s.tablespaceScraper.ScrapeTablespaceMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.coreScraper.ScrapeCoreMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.pdbScraper.ScrapePdbMetrics(ctx)...)
	scrapeErrors = append(scrapeErrors, s.systemScraper.ScrapeSystemMetrics(ctx)...)

	// Collect instance information for individual queries
	instanceInfo, err := s.getInstanceInfo()
	if err != nil {
		s.logger.Warn("Failed to get instance information for individual queries", zap.Error(err))
		// Continue without instance info
		instanceInfo = nil
	}

	// Collect individual query metrics
	var individualQueryConfig *scrapers.IndividualQueryConfig
	if s.config.IndividualQuerySettings != nil {
		individualQueryConfig = &scrapers.IndividualQueryConfig{
			Enabled:        s.config.IndividualQuerySettings.Enabled,
			SearchText:     s.config.IndividualQuerySettings.SearchText,
			ExcludeSchemas: s.config.IndividualQuerySettings.ExcludeSchemas,
			MaxQueries:     s.config.IndividualQuerySettings.MaxQueries,
		}
	}
	individualQueryMetrics, err := s.individualQueryScraper.CollectIndividualQueryMetrics(s.db, []string{}, instanceInfo, individualQueryConfig)
	if err != nil {
		s.logger.Warn("Error collecting individual query metrics", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}

	// Build the resource with instance and host information
	rb := s.mb.NewResourceBuilder()
	rb.SetNewrelicoracledbInstanceName(s.instanceName)
	rb.SetHostName(s.hostName)
	out := s.mb.Emit(metadata.WithResource(rb.Emit()))

	// Merge individual query metrics with standard metrics
	if individualQueryMetrics.ResourceMetrics().Len() > 0 {
		s.logger.Debug("Merging individual query metrics",
			zap.Int("individual_query_resource_metrics", individualQueryMetrics.ResourceMetrics().Len()))
		individualQueryMetrics.ResourceMetrics().MoveAndAppendTo(out.ResourceMetrics())
	}

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

// getInstanceInfo retrieves basic instance information for individual query resource attributes
func (s *newRelicOracleScraper) getInstanceInfo() (*scrapers.InstanceInfo, error) {
	query := `
		SELECT 
			i.INST_ID,
			i.INSTANCE_NAME,
			g.GLOBAL_NAME,
			d.DBID
		FROM 
			gv$instance i,
			global_name g,
			v$database d
		WHERE i.INST_ID = 1`

	row := s.db.QueryRow(query)

	var info scrapers.InstanceInfo
	err := row.Scan(&info.InstanceID, &info.InstanceName, &info.GlobalName, &info.DbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance info: %w", err)
	}

	return &info, nil
}
