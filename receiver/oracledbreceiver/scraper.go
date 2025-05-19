// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

const (
	statsSQL                       = "select * from v$sysstat"
	enqueueDeadlocks               = "enqueue deadlocks"
	exchangeDeadlocks              = "exchange deadlocks"
	executeCount                   = "execute count"
	parseCountTotal                = "parse count (total)"
	parseCountHard                 = "parse count (hard)"
	userCommits                    = "user commits"
	userRollbacks                  = "user rollbacks"
	physicalReads                  = "physical reads"
	physicalReadsDirect            = "physical reads direct"
	physicalReadIORequests         = "physical read IO requests"
	physicalWrites                 = "physical writes"
	physicalWritesDirect           = "physical writes direct"
	physicalWriteIORequests        = "physical write IO requests"
	queriesParallelized            = "queries parallelized"
	ddlStatementsParallelized      = "DDL statements parallelized"
	dmlStatementsParallelized      = "DML statements parallelized"
	parallelOpsNotDowngraded       = "Parallel operations not downgraded"
	parallelOpsDowngradedToSerial  = "Parallel operations downgraded to serial"
	parallelOpsDowngraded1To25Pct  = "Parallel operations downgraded 1 to 25 pct"
	parallelOpsDowngraded25To50Pct = "Parallel operations downgraded 25 to 50 pct"
	parallelOpsDowngraded50To75Pct = "Parallel operations downgraded 50 to 75 pct"
	parallelOpsDowngraded75To99Pct = "Parallel operations downgraded 75 to 99 pct"
	sessionLogicalReads            = "session logical reads"
	cpuTime                        = "CPU used by this session"
	pgaMemory                      = "session pga memory"
	dbBlockGets                    = "db block gets"
	consistentGets                 = "consistent gets"
	sessionCountSQL                = "select status, type, count(*) as VALUE FROM v$session GROUP BY status, type"
	systemResourceLimitsSQL        = "select RESOURCE_NAME, CURRENT_UTILIZATION, LIMIT_VALUE, CASE WHEN TRIM(INITIAL_ALLOCATION) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(INITIAL_ALLOCATION) END as INITIAL_ALLOCATION, CASE WHEN TRIM(LIMIT_VALUE) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(LIMIT_VALUE) END as LIMIT_VALUE from v$resource_limit"
	tablespaceUsageSQL             = `
		select um.TABLESPACE_NAME, um.USED_SPACE, um.TABLESPACE_SIZE, ts.BLOCK_SIZE
		FROM DBA_TABLESPACE_USAGE_METRICS um INNER JOIN DBA_TABLESPACES ts
		ON um.TABLESPACE_NAME = ts.TABLESPACE_NAME`

	dbTimeReferenceFormat = "2006-01-02 15:04:05"
	sqlIDAttr             = "SQL_ID"
	childAddressAttr      = "CHILD_ADDRESS"
	childNumberAttr       = "CHILD_NUMBER"
	sqlTextAttr           = "SQL_FULLTEXT"
	dbPrefix              = "oracledb."
	queryPrefix           = "query."

	queryExecutionMetric        = "EXECUTIONS"
	elapsedTimeMetric           = "ELAPSED_TIME"
	cpuTimeMetric               = "CPU_TIME"
	applicationWaitTimeMetric   = "APPLICATION_WAIT_TIME"
	concurrencyWaitTimeMetric   = "CONCURRENCY_WAIT_TIME"
	userIoWaitTimeMetric        = "USER_IO_WAIT_TIME"
	clusterWaitTimeMetric       = "CLUSTER_WAIT_TIME"
	rowsProcessedMetric         = "ROWS_PROCESSED"
	bufferGetsMetric            = "BUFFER_GETS"
	physicalReadRequestsMetric  = "PHYSICAL_READ_REQUESTS"
	physicalWriteRequestsMetric = "PHYSICAL_WRITE_REQUESTS"
	physicalReadBytesMetric     = "PHYSICAL_READ_BYTES"
	physicalWriteBytesMetric    = "PHYSICAL_WRITE_BYTES"
	queryDiskReadsMetric        = "DISK_READS"
	queryDirectReadsMetric      = "DIRECT_READS"
	queryDirectWritesMetric     = "DIRECT_WRITES"
)

var (

	//go:embed templates/oracleQuerySampleSql.tmpl
	samplesQuery string
	//go:embed templates/oracleQueryMetricsAndTextSql.tmpl
	oracleQueryMetricsSQL string
	//go:embed templates/oracleQueryPlanSql.tmpl
	oracleQueryPlanDataSQL string

	microSecColumnNames = map[string]bool{
		elapsedTimeMetric:         true,
		cpuTimeMetric:             true,
		applicationWaitTimeMetric: true,
		clusterWaitTimeMetric:     true,
		concurrencyWaitTimeMetric: true,
		userIoWaitTimeMetric:      true,
	}

	columnToMetricsMap = map[string]string{
		queryExecutionMetric:        dbPrefix + queryPrefix + "executions",
		elapsedTimeMetric:           dbPrefix + queryPrefix + "elapsed_time",
		cpuTimeMetric:               dbPrefix + queryPrefix + "cpu_time",
		applicationWaitTimeMetric:   dbPrefix + queryPrefix + "application_wait_time",
		concurrencyWaitTimeMetric:   dbPrefix + queryPrefix + "concurrency_wait_time",
		userIoWaitTimeMetric:        dbPrefix + queryPrefix + "user_io_wait_time",
		clusterWaitTimeMetric:       dbPrefix + queryPrefix + "cluster_wait_time",
		rowsProcessedMetric:         dbPrefix + queryPrefix + "rows_processed",
		bufferGetsMetric:            dbPrefix + queryPrefix + "buffer_gets",
		physicalReadRequestsMetric:  dbPrefix + queryPrefix + "physical_read_requests",
		physicalWriteRequestsMetric: dbPrefix + queryPrefix + "physical_write_requests",
		physicalReadBytesMetric:     dbPrefix + queryPrefix + "physical_read_bytes",
		physicalWriteBytesMetric:    dbPrefix + queryPrefix + "physical_write_bytes",
		queryDirectReadsMetric:      dbPrefix + queryPrefix + "direct_reads",
		queryDirectWritesMetric:     dbPrefix + queryPrefix + "direct_writes",
		queryDiskReadsMetric:        dbPrefix + queryPrefix + "disk_reads",
	}
)

type dbProviderFunc func() (*sql.DB, error)

type clientProviderFunc func(*sql.DB, string, *zap.Logger) dbClient

type oracleScraper struct {
	statsClient                dbClient
	tablespaceUsageClient      dbClient
	systemResourceLimitsClient dbClient
	sessionCountClient         dbClient
	samplesQueryClient         dbClient
	oracleQueryMetricsClient   dbClient
	oraclePlanDataClient       dbClient
	db                         *sql.DB
	clientProviderFunc         clientProviderFunc
	mb                         *metadata.MetricsBuilder
	dbProviderFunc             dbProviderFunc
	logger                     *zap.Logger
	id                         component.ID
	instanceName               string
	hostName                   string
	scrapeCfg                  scraperhelper.ControllerConfig
	startTime                  pcommon.Timestamp
	metricsBuilderConfig       metadata.MetricsBuilderConfig
	metricCache                *lru.Cache[string, map[string]int64]
	topQueryCollectCfg         TopQueryCollection
	querySampleCfg             QuerySample
}

func newScraper(metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string) (scraper.Metrics, error) {
	s := &oracleScraper{
		mb:                   metricsBuilder,
		metricsBuilderConfig: metricsBuilderConfig,
		scrapeCfg:            scrapeCfg,
		logger:               logger,
		dbProviderFunc:       providerFunc,
		clientProviderFunc:   clientProviderFunc,
		instanceName:         instanceName,
	}
	return scraper.NewMetrics(s.scrape, scraper.WithShutdown(s.shutdown), scraper.WithStart(s.start))
}

func newLogsScraper(metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig,
	logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string, metricCache *lru.Cache[string, map[string]int64],
	topQueryCollectCfg TopQueryCollection, querySampleCfg QuerySample, hostName string,
) (scraper.Logs, error) {
	s := &oracleScraper{
		metricsBuilderConfig: metricsBuilderConfig,
		scrapeCfg:            scrapeCfg,
		logger:               logger,
		dbProviderFunc:       providerFunc,
		clientProviderFunc:   clientProviderFunc,
		instanceName:         instanceName,
		metricCache:          metricCache,
		topQueryCollectCfg:   topQueryCollectCfg,
		querySampleCfg:       querySampleCfg,
		hostName:             hostName,
	}
	return scraper.NewLogs(s.scrapeLogs, scraper.WithShutdown(s.shutdown), scraper.WithStart(s.start))
}

func (s *oracleScraper) start(context.Context, component.Host) error {
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
	s.samplesQueryClient = s.clientProviderFunc(s.db, samplesQuery, s.logger)
	return nil
}

func (s *oracleScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
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
		s.metricsBuilderConfig.Metrics.OracledbPhysicalReadsDirect.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPhysicalReadIoRequests.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPhysicalWrites.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPhysicalWritesDirect.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbPhysicalWriteIoRequests.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbQueriesParallelized.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbDdlStatementsParallelized.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbDmlStatementsParallelized.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsNotDowngraded.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsDowngradedToSerial.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsDowngraded1To25Pct.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsDowngraded25To50Pct.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsDowngraded50To75Pct.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbParallelOperationsDowngraded75To99Pct.Enabled ||
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
			case physicalReadsDirect:
				err := s.mb.RecordOracledbPhysicalReadsDirectDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case physicalReadIORequests:
				err := s.mb.RecordOracledbPhysicalReadIoRequestsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case physicalWrites:
				err := s.mb.RecordOracledbPhysicalWritesDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case physicalWritesDirect:
				err := s.mb.RecordOracledbPhysicalWritesDirectDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case physicalWriteIORequests:
				err := s.mb.RecordOracledbPhysicalWriteIoRequestsDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case queriesParallelized:
				err := s.mb.RecordOracledbQueriesParallelizedDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case ddlStatementsParallelized:
				err := s.mb.RecordOracledbDdlStatementsParallelizedDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case dmlStatementsParallelized:
				err := s.mb.RecordOracledbDmlStatementsParallelizedDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsNotDowngraded:
				err := s.mb.RecordOracledbParallelOperationsNotDowngradedDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsDowngradedToSerial:
				err := s.mb.RecordOracledbParallelOperationsDowngradedToSerialDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsDowngraded1To25Pct:
				err := s.mb.RecordOracledbParallelOperationsDowngraded1To25PctDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsDowngraded25To50Pct:
				err := s.mb.RecordOracledbParallelOperationsDowngraded25To50PctDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsDowngraded50To75Pct:
				err := s.mb.RecordOracledbParallelOperationsDowngraded50To75PctDataPoint(now, row["VALUE"])
				if err != nil {
					scrapeErrors = append(scrapeErrors, err)
				}
			case parallelOpsDowngraded75To99Pct:
				err := s.mb.RecordOracledbParallelOperationsDowngraded75To99PctDataPoint(now, row["VALUE"])
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

	if s.metricsBuilderConfig.Metrics.OracledbTablespaceSizeUsage.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbTablespaceSizeLimit.Enabled {
		rows, err := s.tablespaceUsageClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", tablespaceUsageSQL, err))
		} else {
			now := pcommon.NewTimestampFromTime(time.Now())
			for _, row := range rows {
				tablespaceName := row["TABLESPACE_NAME"]
				usedSpaceBlockCount, err := strconv.ParseInt(row["USED_SPACE"], 10, 64)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for OracledbTablespaceSizeUsage, value was %s: %w", row["USED_SPACE"], err))
					continue
				}

				tablespaceSizeOriginal := row["TABLESPACE_SIZE"]
				var tablespaceSizeBlockCount int64
				// Tablespace size should never be empty using the DBA_TABLESPACE_USAGE_METRICS query. This logic is done
				// to preserve backward compatibility for with the original metric gathered from querying DBA_TABLESPACES
				if tablespaceSizeOriginal == "" {
					tablespaceSizeBlockCount = -1
				} else {
					tablespaceSizeBlockCount, err = strconv.ParseInt(tablespaceSizeOriginal, 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for OracledbTablespaceSizeLimit, value was %s: %w", tablespaceSizeOriginal, err))
						continue
					}
				}

				blockSize, err := strconv.ParseInt(row["BLOCK_SIZE"], 10, 64)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for OracledbBlockSize, value was %s: %w", row["BLOCK_SIZE"], err))
					continue
				}

				s.mb.RecordOracledbTablespaceSizeUsageDataPoint(now, usedSpaceBlockCount*blockSize, tablespaceName)

				if tablespaceSizeBlockCount < 0 {
					s.mb.RecordOracledbTablespaceSizeLimitDataPoint(now, -1, tablespaceName)
				} else {
					s.mb.RecordOracledbTablespaceSizeLimitDataPoint(now, tablespaceSizeBlockCount*blockSize, tablespaceName)
				}
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

type queryMetricCacheHit struct {
	sqlID        string
	childNumber  string
	childAddress string
	queryText    string
	metrics      map[string]int64
}

func (s *oracleScraper) scrapeLogs(ctx context.Context) (plog.Logs, error) {
	logs := plog.NewLogs()
	var scrapeErrors []error

	if s.topQueryCollectCfg.Enabled {
		topNCollectionErr := s.collectTopNMetricData(ctx, logs)
		if topNCollectionErr != nil {
			scrapeErrors = append(scrapeErrors, topNCollectionErr)
		}
	}

	if s.querySampleCfg.Enabled {
		samplesCollectionErrors := s.collectQuerySamples(ctx, logs)
		if samplesCollectionErrors != nil {
			scrapeErrors = append(scrapeErrors, samplesCollectionErrors)
		}
	}

	return logs, errors.Join(scrapeErrors...)
}

func (s *oracleScraper) collectQuerySamples(ctx context.Context, logs plog.Logs) error {
	const duration = "DURATION_SEC"
	const event = "EVENT"
	const hostName = "MACHINE"
	const module = "MODULE"
	const osUser = "OSUSER"
	const objectName = "OBJECT_NAME"
	const objectType = "OBJECT_TYPE"
	const process = "PROCESS"
	const program = "PROGRAM"
	const planHashValue = "PLAN_HASH_VALUE"
	const sqlID = "SQL_ID"
	const schemaName = "SCHEMANAME"
	const sqlChildNumber = "SQL_CHILD_NUMBER"
	const sid = "SID"
	const serialNumber = "SERIAL"
	const status = "STATUS"
	const state = "STATE"
	const sqlText = "SQL_FULLTEXT"
	const username = "USERNAME"
	const waitclass = "WAIT_CLASS"

	var scrapeErrors []error

	dbClients := s.samplesQueryClient

	rows, err := dbClients.metricRows(ctx)
	if err != nil {
		scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", samplesQuery, err))
	}

	resourceLog := logs.ResourceLogs().AppendEmpty()
	resourceAttributes := resourceLog.Resource().Attributes()

	resourceAttributes.PutStr(dbPrefix+"instance.name", s.instanceName)
	resourceAttributes.PutStr("host.name", s.hostName)

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	for _, row := range rows {
		obfuscatedSQL, err := ObfuscateSQL(row[sqlText])
		if err != nil {
			s.logger.Error(fmt.Sprintf("oracleScraper failed getting metric row: %s", err))
			continue
		}

		queryPlanHashVal := hex.EncodeToString([]byte(row[planHashValue]))
		record := scopedLog.LogRecords().AppendEmpty()
		record.SetEventName("db.server.query_sample")
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		record.Attributes().PutStr(strings.ToLower("db.query.text"), obfuscatedSQL)

		record.Attributes().PutStr("db.system.name", "oracle")
		// reporting human-readable query  hash plan
		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+planHashValue), queryPlanHashVal)

		record.Attributes().PutStr(strings.ToLower(dbPrefix+hostName), row[hostName])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"id"), row[sqlID])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"child_number"), row[sqlChildNumber])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+sid), row[sid])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"serial_number"), row[serialNumber])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+process), row[process])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"id"), row[sqlID])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"child_number"), row[sqlChildNumber])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+sid), row[sid])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+"serial_number"), row[serialNumber])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+process), row[process])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+username), row[username])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+schemaName), row[schemaName])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+program), row[program])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+module), row[module])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+status), row[status])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+state), row[state])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+waitclass), row[waitclass])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+event), row[event])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+objectName), row[objectName])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+objectType), row[objectType])

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+osUser), row[osUser])

		i, err := strconv.ParseFloat(row[duration], 64)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for Duration, value was %s: %w", row[duration], err))
		}
		record.Attributes().PutDouble(dbPrefix+queryPrefix+"duration", i)
	}

	return errors.Join(scrapeErrors...)
}

func (s *oracleScraper) collectTopNMetricData(ctx context.Context, logs plog.Logs) error {
	var errs []error
	// get metrics and query texts from DB
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	intervalSeconds := int(s.scrapeCfg.CollectionInterval.Seconds())
	s.oracleQueryMetricsClient = s.clientProviderFunc(s.db, oracleQueryMetricsSQL, s.logger)
	now := timestamp.AsTime().Format(dbTimeReferenceFormat)
	metricRows, metricError := s.oracleQueryMetricsClient.metricRows(ctx, now, intervalSeconds, s.topQueryCollectCfg.MaxQuerySampleCount)

	if metricError != nil {
		errs = append(errs, fmt.Errorf("error executing %s: %w", oracleQueryMetricsSQL, metricError))
		return errors.Join(errs...)
	}

	if len(metricRows) == 0 {
		errs = append(errs, errors.New("no data returned from oracleQueryMetricsClient"))
		return errors.Join(errs...)
	}

	enabledColumns := s.getEnabledMetricColumns()
	s.logger.Debug("Enabled metric columns", zap.Strings("names", enabledColumns))
	s.logger.Debug("Cache", zap.Int("size", s.metricCache.Len()))
	var hits []queryMetricCacheHit
	var cacheUpdates, discardedHits int
	for _, row := range metricRows {
		newCacheVal := make(map[string]int64, len(enabledColumns))
		for _, columnName := range enabledColumns {
			val := row[columnName]
			valInt64, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				errs = append(errs, err)
			} else {
				newCacheVal[columnName] = valInt64
			}
		}

		cacheKey := fmt.Sprintf("%v:%v", row[sqlIDAttr], row[childNumberAttr])
		// if we have a cache hit and the query doesn't belong to top N, cache is updated anyway
		// as a result, once it finally makes its way to the top N queries, only the latest delta will be sent downstream
		if oldCacheVal, ok := s.metricCache.Get(cacheKey); ok {
			hit := queryMetricCacheHit{
				sqlID:        row[sqlIDAttr],
				queryText:    row[sqlTextAttr],
				childNumber:  row[childNumberAttr],
				childAddress: row[childAddressAttr],
				metrics:      make(map[string]int64, len(enabledColumns)),
			}

			// it is possible we get a record with all deltas equal to zero. we don't want to process it any further
			var possiblePurge, positiveDelta bool
			for _, columnName := range enabledColumns {
				delta := newCacheVal[columnName] - oldCacheVal[columnName]

				// if any of the deltas is less than zero, cursor was likely purged from the shared pool
				if delta < 0 {
					possiblePurge = true
					break
				} else if delta > 0 {
					positiveDelta = true
				}

				hit.metrics[columnName] = delta
			}

			// skip if possible purge or all the deltas are equal to zero
			if !possiblePurge && positiveDelta {
				hits = append(hits, hit)
			} else {
				discardedHits++
			}
		}
		s.metricCache.Add(cacheKey, newCacheVal)
		cacheUpdates++
	}

	// if cache updates is not equal to rows returned, that indicates there is problem somewhere
	s.logger.Debug("Cache update", zap.Int("update-count", cacheUpdates), zap.Int("new-size", s.metricCache.Len()))

	if len(hits) == 0 {
		s.logger.Info("No log records for this scrape")
		return errors.Join(errs...)
	}

	s.logger.Info("Cache hits", zap.Int("hit-count", len(hits)), zap.Int("discarded-hit-count", discardedHits))
	for _, hit := range hits {
		s.logger.Debug(fmt.Sprintf("Cache hit, SQL_ID: %v, CHILD_NUMBER: %v", hit.sqlID, hit.childNumber), zap.Int64("elapsed-time", hit.metrics[elapsedTimeMetric]))
	}

	// order by elapsed time delta, descending
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].metrics[elapsedTimeMetric] > hits[j].metrics[elapsedTimeMetric]
	})

	for _, hit := range hits {
		s.logger.Debug(fmt.Sprintf("Cache hit after sorting, SQL_ID: %v, CHILD_NUMBER: %v", hit.sqlID, hit.childNumber), zap.String("child-address", hit.childAddress), zap.Int64("elapsed-time", hit.metrics[elapsedTimeMetric]))
	}

	// keep at most maxHitSize
	hitCountBefore := len(hits)
	maxHitsSize := min(len(hits), int(s.topQueryCollectCfg.TopQueryCount))
	hits = hits[:maxHitsSize]
	skippedCacheHits := hitCountBefore - len(hits)
	s.logger.Debug("Skipped cache hits", zap.Int("count", skippedCacheHits))

	for _, hit := range hits {
		s.logger.Debug(fmt.Sprintf("Final cache hit, SQL_ID: %v, CHILD_NUMBER: %v", hit.sqlID, hit.childNumber), zap.String("child-address", hit.childAddress), zap.Any("metrics", hit.metrics))
	}
	hits = s.obfuscateCacheHits(hits)
	childAddressToPlanMap := s.getChildAddressToPlanMap(ctx, hits)

	topNResourceLog := logs.ResourceLogs().AppendEmpty()

	resourceAttributes := topNResourceLog.Resource().Attributes()
	if s.metricsBuilderConfig.ResourceAttributes.OracledbInstanceName.Enabled {
		resourceAttributes.PutStr(dbPrefix+"instance.name", s.instanceName)
	}
	resourceAttributes.PutStr("host.name", s.hostName)

	scopedLog := topNResourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	for _, hit := range hits {
		record := scopedLog.LogRecords().AppendEmpty()
		record.SetTimestamp(timestamp)
		record.SetEventName("db.server.top_query")

		for _, columnName := range enabledColumns {
			columnValue := hit.metrics[columnName]
			if microSecColumnNames[columnName] {
				record.Attributes().PutDouble(columnToMetricsMap[columnName], float64(columnValue)/1_000_000)
			} else {
				record.Attributes().PutInt(columnToMetricsMap[columnName], columnValue)
			}
		}
		if record.Attributes().Len() == 0 {
			continue
		}

		planBytes, err := json.Marshal(childAddressToPlanMap[hit.childAddress])
		if err != nil {
			s.logger.Error("Error marshaling plan data to JSON", zap.Error(err))
		}
		planString := string(planBytes)
		// if there is no execution plan, we send an empty string
		record.Attributes().PutStr(dbPrefix+"query_plan", planString)
		record.Attributes().PutStr("db."+queryPrefix+"text", hit.queryText)
		record.Attributes().PutStr(dbPrefix+queryPrefix+"sql_id", hit.sqlID)
		record.Attributes().PutStr(dbPrefix+queryPrefix+"child_number", hit.childNumber)
		record.Attributes().PutStr("db.system.name", "oracle")
	}

	hitCount := len(hits)
	if hitCount > 0 {
		s.logger.Debug("Log records for this scrape", zap.Int("count", hitCount))
	}

	return errors.Join(errs...)
}

func (s *oracleScraper) obfuscateCacheHits(hits []queryMetricCacheHit) []queryMetricCacheHit {
	var obfuscatedHits []queryMetricCacheHit
	for _, hit := range hits {
		// obfuscate and normalize the query text
		obfuscatedSQL, err := ObfuscateSQL(hit.queryText)
		if err != nil {
			s.logger.Error("oracleScraper failed getting metric rows", zap.Error(err))
		} else {
			obfuscatedSQLLowerCase := strings.ToLower(obfuscatedSQL)
			hit.queryText = obfuscatedSQLLowerCase
			obfuscatedHits = append(obfuscatedHits, hit)
		}
	}
	return obfuscatedHits
}

func (s *oracleScraper) getChildAddressToPlanMap(ctx context.Context, hits []queryMetricCacheHit) map[string][]metricRow {
	childAddressToPlanMap := map[string][]metricRow{}
	if len(hits) == 0 {
		return childAddressToPlanMap
	}

	var childAddressSlice []any
	placeholders := make([]string, len(hits))
	for i, hit := range hits {
		placeholders[i] = fmt.Sprintf("HEXTORAW(:%d)", i+1)
		childAddressSlice = append(childAddressSlice, hit.childAddress)
	}

	placeholdersCombined := strings.Join(placeholders, ", ")
	sqlQuery := fmt.Sprintf(oracleQueryPlanDataSQL, placeholdersCombined)

	s.logger.Debug("Fetching execution plans")
	s.oraclePlanDataClient = s.clientProviderFunc(s.db, sqlQuery, s.logger)
	planData, _ := s.oraclePlanDataClient.metricRows(ctx, childAddressSlice...)

	for _, row := range planData {
		currentChildAddress := row[childAddressAttr]
		jsonPlansSlice, ok := childAddressToPlanMap[currentChildAddress]
		// child address was for internal use only, it's not going to be used beyond this point
		delete(row, childAddressAttr)
		if ok {
			childAddressToPlanMap[currentChildAddress] = append(jsonPlansSlice, row)
		} else {
			childAddressToPlanMap[currentChildAddress] = []metricRow{row}
		}
	}

	return childAddressToPlanMap
}

func (s *oracleScraper) getEnabledMetricColumns() []string {
	// This function will later be extended to read enabled metrics from configuration once mdatagen can support it.
	enabledColumns := []string{
		elapsedTimeMetric, queryExecutionMetric, cpuTimeMetric, applicationWaitTimeMetric,
		concurrencyWaitTimeMetric, userIoWaitTimeMetric, clusterWaitTimeMetric, rowsProcessedMetric, bufferGetsMetric,
		physicalReadRequestsMetric, physicalWriteRequestsMetric, physicalReadBytesMetric, physicalWriteBytesMetric,
		queryDirectReadsMetric, queryDirectWritesMetric, queryDiskReadsMetric,
	}
	return enabledColumns
}

func (s *oracleScraper) shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
