// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

const (
	statsSQL                = "select * from v$sysstat"
	enqueueDeadlocks        = "enqueue deadlocks"
	exchangeDeadlocks       = "exchange deadlocks"
	executeCount            = "execute count"
	parseCountTotal         = "parse count (total)"
	parseCountHard          = "parse count (hard)"
	userCommits             = "user commits"
	userRollbacks           = "user rollbacks"
	physicalReads           = "physical reads"
	sessionLogicalReads     = "session logical reads"
	cpuTime                 = "CPU used by this session"
	pgaMemory               = "session pga memory"
	dbBlockGets             = "db block gets"
	consistentGets          = "consistent gets"
	sessionCountSQL         = "select status, type, count(*) as VALUE FROM v$session GROUP BY status, type"
	systemResourceLimitsSQL = "select RESOURCE_NAME, CURRENT_UTILIZATION, LIMIT_VALUE, CASE WHEN TRIM(INITIAL_ALLOCATION) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(INITIAL_ALLOCATION) END as INITIAL_ALLOCATION, CASE WHEN TRIM(LIMIT_VALUE) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(LIMIT_VALUE) END as LIMIT_VALUE from v$resource_limit"
	tablespaceUsageSQL      = "select TABLESPACE_NAME, BYTES from DBA_DATA_FILES"
	tablespaceMaxSpaceSQL   = "select TABLESPACE_NAME, (BLOCK_SIZE*MAX_EXTENTS) AS VALUE FROM DBA_TABLESPACES"
)

type dbProviderFunc func() (*sql.DB, error)

type clientProviderFunc func(*sql.DB, string, *zap.Logger) dbClient

type scraper struct {
	statsClient                dbClient
	tablespaceMaxSpaceClient   dbClient
	tablespaceUsageClient      dbClient
	systemResourceLimitsClient dbClient
	sessionCountClient         dbClient
	db                         *sql.DB
	clientProviderFunc         clientProviderFunc
	mb                         *metadata.MetricsBuilder
	dbProviderFunc             dbProviderFunc
	logger                     *zap.Logger
	id                         component.ID
	instanceName               string
	scrapeCfg                  scraperhelper.ScraperControllerSettings
	startTime                  pcommon.Timestamp
	metricsBuilderConfig       metadata.MetricsBuilderConfig
}

func newScraper(id component.ID, metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ScraperControllerSettings, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string) (scraperhelper.Scraper, error) {
	s := &scraper{
		mb:                   metricsBuilder,
		metricsBuilderConfig: metricsBuilderConfig,
		scrapeCfg:            scrapeCfg,
		logger:               logger,
		dbProviderFunc:       providerFunc,
		clientProviderFunc:   clientProviderFunc,
		instanceName:         instanceName,
	}
	return scraperhelper.NewScraper(id.String(), s.scrape, scraperhelper.WithShutdown(s.shutdown), scraperhelper.WithStart(s.start))
}

func (s *scraper) start(context.Context, component.Host) error {
	s.startTime = pcommon.NewTimestampFromTime(time.Now())
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	s.statsClient = s.clientProviderFunc(s.db, statsSQL, s.logger)
	s.sessionCountClient = s.clientProviderFunc(s.db, sessionCountSQL, s.logger)
	s.systemResourceLimitsClient = s.clientProviderFunc(s.db, systemResourceLimitsSQL, s.logger)
	s.tablespaceUsageClient = s.clientProviderFunc(s.db, tablespaceUsageSQL, s.logger)
	s.tablespaceMaxSpaceClient = s.clientProviderFunc(s.db, tablespaceMaxSpaceSQL, s.logger)
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin scrape")

	var scrapeErrors []error

	runStats := s.metricsBuilderConfig.Metrics.OracledbEnqueueDeadlocks.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbExchangeDeadlocks.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbExecutions.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParseCalls.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbHardParses.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbUserCommits.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbUserRollbacks.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPhysicalReads.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbLogicalReads.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbCPUTime.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPgaMemory.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbDbBlockGets.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbConsistentGets.Enabled
	if runStats {
		now := pcommon.NewTimestampFromTime(time.Now())
		rows, execError := s.statsClient.metricRows(ctx)
		if execError != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", statsSQL, execError))
		}

		for _, row := range rows {
			switch row["NAME"] {
			case enqueueDeadlocks:
				err := s.mb.RecordOracledbEnqueueDeadlocksDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case exchangeDeadlocks:
				err := s.mb.RecordOracledbExchangeDeadlocksDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case executeCount:
				err := s.mb.RecordOracledbExecutionsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parseCountTotal:
				err := s.mb.RecordOracledbParseCallsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parseCountHard:
				err := s.mb.RecordOracledbHardParsesDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case userCommits:
				err := s.mb.RecordOracledbUserCommitsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case userRollbacks:
				err := s.mb.RecordOracledbUserRollbacksDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case physicalReads:
				err := s.mb.RecordOracledbPhysicalReadsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case sessionLogicalReads:
				err := s.mb.RecordOracledbLogicalReadsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case cpuTime:
				value, err := strconv.ParseFloat(row["VALUE"], 64)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", cpuTime, row["VALUE"], err))
				} else {
					// divide by 100 as the value is expressed in tens of milliseconds
					value /= 100
					s.mb.RecordOracledbCPUTimeDataPoint(now, value)
				}
			case pgaMemory:
				err := s.mb.RecordOracledbPgaMemoryDataPoint(pcommon.NewTimestampFromTime(time.Now()), row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case dbBlockGets:
				err := s.mb.RecordOracledbDbBlockGetsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case consistentGets:
				err := s.mb.RecordOracledbConsistentGetsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			}
		}
	}

	if s.metricsBuilderConfig.Metrics.OracledbSessionsUsage.Enabled {
		rows, err := s.sessionCountClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", sessionCountSQL, err))
		}
		for _, row := range rows {
			err := s.mb.RecordOracledbSessionsUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), row["VALUE"],
				row["TYPE"], row["STATUS"])
			if err != nil {
				scrapeErrors = append(scrapeErrors, err)
			}
		}
	}

	if s.metricsBuilderConfig.Metrics.OracledbSessionsLimit.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbProcessesUsage.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbProcessesLimit.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbEnqueueResourcesUsage.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbEnqueueResourcesLimit.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbEnqueueLocksLimit.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbEnqueueLocksUsage.Enabled {
		rows, err := s.systemResourceLimitsClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", systemResourceLimitsSQL, err))
		}
		for _, row := range rows {
			resourceName := row["RESOURCE_NAME"]
			switch resourceName {
			case "processes":
				if err := s.mb.RecordOracledbProcessesUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["CURRENT_UTILIZATION"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
				if err := s.mb.RecordOracledbProcessesLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case "sessions":
				err := s.mb.RecordOracledbSessionsLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case "enqueue_locks":
				if err := s.mb.RecordOracledbEnqueueLocksUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["CURRENT_UTILIZATION"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
				if err := s.mb.RecordOracledbEnqueueLocksLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case "dml_locks":
				if err := s.mb.RecordOracledbDmlLocksUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["CURRENT_UTILIZATION"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
				if err := s.mb.RecordOracledbDmlLocksLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case "enqueue_resources":
				if err := s.mb.RecordOracledbEnqueueResourcesUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["CURRENT_UTILIZATION"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
				if err := s.mb.RecordOracledbEnqueueResourcesLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case "transactions":
				if err := s.mb.RecordOracledbTransactionsUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["CURRENT_UTILIZATION"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
				if err := s.mb.RecordOracledbTransactionsLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()),
					row["LIMIT_VALUE"]); err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			}
		}
	}
	if s.metricsBuilderConfig.Metrics.OracledbTablespaceSizeUsage.Enabled {
		rows, err := s.tablespaceUsageClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", tablespaceUsageSQL, err))
		} else {
			now := pcommon.NewTimestampFromTime(time.Now())
			for _, row := range rows {
				tablespaceName := row["TABLESPACE_NAME"]
				err := s.mb.RecordOracledbTablespaceSizeUsageDataPoint(now, row["BYTES"], tablespaceName)
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			}
		}
	}
	if s.metricsBuilderConfig.Metrics.OracledbTablespaceSizeLimit.Enabled {
		rows, err := s.tablespaceMaxSpaceClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", tablespaceMaxSpaceSQL, err))
		} else {
			now := pcommon.NewTimestampFromTime(time.Now())
			for _, row := range rows {
				tablespaceName := row["TABLESPACE_NAME"]
				var val int64
				inputVal := row["VALUE"]
				if inputVal == "" {
					val = -1
				} else {
					val, err = strconv.ParseInt(inputVal, 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for OracledbTablespaceSizeLimit, value was %s: %w", inputVal, err))
						continue
					}
				}
				s.mb.RecordOracledbTablespaceSizeLimitDataPoint(now, val, tablespaceName)
			}
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetOracledbInstanceName(s.instanceName)
	out := s.mb.Emit(metadata.WithResource(rb.Emit()))
	s.logger.Debug("Done scraping")
	if len(scrapeErrors) > 0 {
		return out, scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return out, nil
}

func (s *scraper) shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
