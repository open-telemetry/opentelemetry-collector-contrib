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
	"math"
	"net"
	"os"
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
	"go.opentelemetry.io/otel/propagation"
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
	logons                         = "logons cumulative"
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

	sqlIDAttr        = "SQL_ID"
	childAddressAttr = "CHILD_ADDRESS"
	childNumberAttr  = "CHILD_NUMBER"
	sqlTextAttr      = "SQL_FULLTEXT"
	dbSystemNameVal  = "oracle"

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

	// Stored procedure columns
	objectIDAttr   = "PROGRAM_ID"
	objectNameAttr = "PROCEDURE_NAME"
	objectTypeAttr = "PROCEDURE_TYPE"
)

var (
	//go:embed templates/oracleQuerySampleSql.tmpl
	samplesQuery string
	//go:embed templates/oracleQueryMetricsAndTextSql.tmpl
	oracleQueryMetricsSQL string
	//go:embed templates/oracleQueryPlanSql.tmpl
	oracleQueryPlanDataSQL string
)

type dbProviderFunc func() (*sql.DB, error)

type clientProviderFunc func(*sql.DB, string, *zap.Logger) dbClient

type oracleScraper struct {
	statsClient                dbClient
	tablespaceUsageClient      dbClient
	systemResourceLimitsClient dbClient
	sessionCountClient         dbClient
	oracleQueryMetricsClient   dbClient
	oraclePlanDataClient       dbClient
	samplesQueryClient         dbClient
	db                         *sql.DB
	clientProviderFunc         clientProviderFunc
	mb                         *metadata.MetricsBuilder
	lb                         *metadata.LogsBuilder
	dbProviderFunc             dbProviderFunc
	logger                     *zap.Logger
	id                         component.ID
	instanceName               string
	hostName                   string
	scrapeCfg                  scraperhelper.ControllerConfig
	startTime                  pcommon.Timestamp
	metricsBuilderConfig       metadata.MetricsBuilderConfig
	logsBuilderConfig          metadata.LogsBuilderConfig
	metricCache                *lru.Cache[string, map[string]int64]
	topQueryCollectCfg         TopQueryCollection
	obfuscator                 *obfuscator
	querySampleCfg             QuerySample
	serviceInstanceID          string
	lastExecutionTimestamp     time.Time
}

func newScraper(metricsBuilder *metadata.MetricsBuilder, metricsBuilderConfig metadata.MetricsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName, hostName string) (scraper.Metrics, error) {
	s := &oracleScraper{
		mb:                   metricsBuilder,
		metricsBuilderConfig: metricsBuilderConfig,
		scrapeCfg:            scrapeCfg,
		logger:               logger,
		dbProviderFunc:       providerFunc,
		clientProviderFunc:   clientProviderFunc,
		instanceName:         instanceName,
		hostName:             hostName,
		serviceInstanceID:    getInstanceID(instanceName, logger),
	}
	return scraper.NewMetrics(s.scrape, scraper.WithShutdown(s.shutdown), scraper.WithStart(s.start))
}

func newLogsScraper(logsBuilder *metadata.LogsBuilder, logsBuilderConfig metadata.LogsBuilderConfig, scrapeCfg scraperhelper.ControllerConfig,
	logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string, metricCache *lru.Cache[string, map[string]int64],
	topQueryCollectCfg TopQueryCollection, querySampleCfg QuerySample, hostName string,
) (scraper.Logs, error) {
	s := &oracleScraper{
		lb:                 logsBuilder,
		logsBuilderConfig:  logsBuilderConfig,
		scrapeCfg:          scrapeCfg,
		logger:             logger,
		dbProviderFunc:     providerFunc,
		clientProviderFunc: clientProviderFunc,
		instanceName:       instanceName,
		metricCache:        metricCache,
		topQueryCollectCfg: topQueryCollectCfg,
		querySampleCfg:     querySampleCfg,
		hostName:           hostName,
		obfuscator:         newObfuscator(),
		serviceInstanceID:  getInstanceID(instanceName, logger),
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
		s.metricsBuilderConfig.Metrics.OracledbConsistentGets.Enabled ||
		s.metricsBuilderConfig.Metrics.OracledbLogons.Enabled
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
			case logons:
				err := s.mb.RecordOracledbLogonsDataPoint(now, row["VALUE"])
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

	rb := s.setupResourceBuilder(s.mb.NewResourceBuilder())

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
	objectID     int64
	objectName   string
	objectType   string
}

func (s *oracleScraper) scrapeLogs(ctx context.Context) (plog.Logs, error) {
	logs := plog.NewLogs()
	var scrapeErrors []error

	if s.logsBuilderConfig.Events.DbServerTopQuery.Enabled {
		currentCollectionTime := time.Now()
		lookbackTimeCounter := s.calculateLookbackSeconds()
		if lookbackTimeCounter < int(s.topQueryCollectCfg.CollectionInterval.Seconds()) {
			s.logger.Debug("Skipping the collection of top queries because collection interval has not yet elapsed.")
		} else {
			topNCollectionErrors := s.collectTopNMetricData(ctx, logs, currentCollectionTime, lookbackTimeCounter)
			if topNCollectionErrors != nil {
				scrapeErrors = append(scrapeErrors, topNCollectionErrors)
			}
			s.lastExecutionTimestamp = currentCollectionTime
		}
	}

	if s.logsBuilderConfig.Events.DbServerQuerySample.Enabled {
		samplesCollectionErrors := s.collectQuerySamples(ctx, logs)
		if samplesCollectionErrors != nil {
			scrapeErrors = append(scrapeErrors, samplesCollectionErrors)
		}
	}

	return logs, errors.Join(scrapeErrors...)
}

func (s *oracleScraper) collectTopNMetricData(ctx context.Context, logs plog.Logs, collectionTime time.Time, lookbackTimeSeconds int) error {
	var errs []error
	// get metrics and query texts from DB
	s.oracleQueryMetricsClient = s.clientProviderFunc(s.db, oracleQueryMetricsSQL, s.logger)
	metricRows, metricError := s.oracleQueryMetricsClient.metricRows(ctx, lookbackTimeSeconds, s.topQueryCollectCfg.MaxQuerySampleCount)

	if metricError != nil {
		return fmt.Errorf("error executing oracleQueryMetricsSQL: %w", metricError)
	}
	if len(metricRows) == 0 {
		return errors.New("no data returned from oracleQueryMetricsClient")
	}

	metricNames := s.getTopNMetricNames()
	var hits []queryMetricCacheHit
	var cacheUpdates, discardedHits int
	for _, row := range metricRows {
		newCacheVal := make(map[string]int64, len(metricNames))
		for _, columnName := range metricNames {
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
			// Parse stored procedure PROGRAM_ID
			var objectID int64
			if row[objectIDAttr] != "" {
				objectID, _ = strconv.ParseInt(row[objectIDAttr], 10, 64)
			}

			hit := queryMetricCacheHit{
				sqlID:        row[sqlIDAttr],
				queryText:    row[sqlTextAttr],
				childNumber:  row[childNumberAttr],
				childAddress: row[childAddressAttr],
				metrics:      make(map[string]int64, len(metricNames)),
				objectID:     objectID,
				objectName:   row[objectNameAttr],
				objectType:   row[objectTypeAttr],
			}

			// it is possible we get a record with all deltas equal to zero. we don't want to process it any further
			var possiblePurge, positiveDelta bool
			for _, columnName := range metricNames {
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

	s.logger.Debug("Cache hits", zap.Int("hit-count", len(hits)), zap.Int("discarded-hit-count", discardedHits))

	// order by elapsed time delta, descending
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].metrics[elapsedTimeMetric] > hits[j].metrics[elapsedTimeMetric]
	})

	// keep at most maxHitSize
	maxHitsSize := min(len(hits), int(s.topQueryCollectCfg.TopQueryCount))
	hits = hits[:maxHitsSize]

	hits = s.obfuscateCacheHits(hits)
	childAddressToPlanMap := s.getChildAddressToPlanMap(ctx, hits)

	rb := s.setupResourceBuilder(s.lb.NewResourceBuilder())

	for _, hit := range hits {
		planBytes, err := json.Marshal(childAddressToPlanMap[hit.childAddress])
		if err != nil {
			s.logger.Error("Error marshaling plan data to JSON", zap.Error(err))
		}
		planString := string(planBytes)

		s.lb.RecordDbServerTopQueryEvent(context.Background(),
			pcommon.NewTimestampFromTime(collectionTime),
			dbSystemNameVal,
			s.hostName,
			hit.queryText,
			planString, hit.sqlID, hit.childNumber,
			hit.childAddress,
			asFloatInSeconds(hit.metrics[applicationWaitTimeMetric]),
			hit.metrics[bufferGetsMetric],
			asFloatInSeconds(hit.metrics[clusterWaitTimeMetric]),
			asFloatInSeconds(hit.metrics[concurrencyWaitTimeMetric]),
			asFloatInSeconds(hit.metrics[cpuTimeMetric]),
			hit.metrics[queryDirectReadsMetric],
			hit.metrics[queryDirectWritesMetric],
			hit.metrics[queryDiskReadsMetric],
			asFloatInSeconds(hit.metrics[elapsedTimeMetric]),
			hit.metrics[queryExecutionMetric],
			hit.metrics[physicalReadBytesMetric],
			hit.metrics[physicalReadRequestsMetric],
			hit.metrics[physicalWriteBytesMetric],
			hit.metrics[physicalWriteRequestsMetric],
			hit.metrics[rowsProcessedMetric],
			asFloatInSeconds(hit.metrics[userIoWaitTimeMetric]),
			hit.objectID,
			hit.objectName,
			hit.objectType)
	}

	hitCount := len(hits)
	if hitCount > 0 {
		s.logger.Debug("Log records for this scrape", zap.Int("count", hitCount))
	}

	s.lb.Emit(metadata.WithLogsResource(rb.Emit())).ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())

	return errors.Join(errs...)
}

func (s *oracleScraper) collectQuerySamples(ctx context.Context, logs plog.Logs) error {
	const action = "ACTION"
	const duration = "DURATION_SEC"
	const event = "EVENT"
	const hostName = "MACHINE"
	const module = "MODULE"
	const osUser = "OSUSER"
	const objectID = "PROCEDURE_ID"
	const objectName = "PROCEDURE_NAME"
	const objectType = "PROCEDURE_TYPE"
	const process = "PROCESS"
	const program = "PROGRAM"
	const planHashValue = "PLAN_HASH_VALUE"
	const sqlID = "SQL_ID"
	const schemaName = "SCHEMANAME"
	const sqlChildNumber = "SQL_CHILD_NUMBER"
	const childAddress = "CHILD_ADDRESS"
	const sid = "SID"
	const serialNumber = "SERIAL#"
	const status = "STATUS"
	const state = "STATE"
	const sqlText = "SQL_FULLTEXT"
	const username = "USERNAME"
	const waitclass = "WAIT_CLASS"
	const port = "PORT"
	const serviceName = "SERVICE_NAME"

	var scrapeErrors []error

	dbClients := s.samplesQueryClient
	propagator := propagation.TraceContext{}
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	rows, err := dbClients.metricRows(ctx, s.querySampleCfg.MaxRowsPerQuery)
	if err != nil {
		scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", samplesQuery, err))
	}

	rb := s.setupResourceBuilder(s.lb.NewResourceBuilder())

	for _, row := range rows {
		if row[sqlText] == "" {
			continue
		}

		obfuscatedSQL, err := s.obfuscator.obfuscateSQLString(row[sqlText])
		if err != nil {
			s.logger.Error(fmt.Sprintf("oracleScraper failed updating this log record: %s", err))
			continue
		}

		queryPlanHashVal := hex.EncodeToString([]byte(row[planHashValue]))

		queryDuration, err := strconv.ParseFloat(row[duration], 64)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for Duration, value was %s: %w", row[duration], err))
		}

		clientPort, err := strconv.ParseInt(row[port], 10, 64)
		if err != nil {
			clientPort = 0
		}

		// Parse stored procedure PROCEDURE_ID
		var objID int64
		if row[objectID] != "" {
			objID, _ = strconv.ParseInt(row[objectID], 10, 64)
		}

		queryContext := propagator.Extract(context.Background(), propagation.MapCarrier{
			"traceparent": row[action],
		})

		s.lb.RecordDbServerQuerySampleEvent(queryContext, timestamp, obfuscatedSQL, dbSystemNameVal, row[username], row[serviceName], row[hostName],
			clientPort, row[hostName], clientPort, queryPlanHashVal, row[sqlID], row[sqlChildNumber], row[childAddress], row[sid], row[serialNumber], row[process],
			row[schemaName], row[program], row[module], row[status], row[state], row[waitclass], row[event], objID, row[objectName], row[objectType],
			row[osUser], queryDuration)
	}

	s.lb.Emit(metadata.WithLogsResource(rb.Emit())).ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())

	return errors.Join(scrapeErrors...)
}

func asFloatInSeconds(value int64) float64 {
	return float64(value) / 1_000_000
}

func (s *oracleScraper) obfuscateCacheHits(hits []queryMetricCacheHit) []queryMetricCacheHit {
	var obfuscatedHits []queryMetricCacheHit
	for _, hit := range hits {
		// obfuscate and normalize the query text
		obfuscatedSQL, err := s.obfuscator.obfuscateSQLString(hit.queryText)
		if err != nil {
			s.logger.Error("oracleScraper failed getting metric rows", zap.Error(err))
		} else {
			hit.queryText = obfuscatedSQL
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

func (*oracleScraper) getTopNMetricNames() []string {
	return []string{
		elapsedTimeMetric, queryExecutionMetric, cpuTimeMetric, applicationWaitTimeMetric,
		concurrencyWaitTimeMetric, userIoWaitTimeMetric, clusterWaitTimeMetric, rowsProcessedMetric, bufferGetsMetric,
		physicalReadRequestsMetric, physicalWriteRequestsMetric, physicalReadBytesMetric, physicalWriteBytesMetric,
		queryDirectReadsMetric, queryDirectWritesMetric, queryDiskReadsMetric,
	}
}

func (s *oracleScraper) shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *oracleScraper) setupResourceBuilder(rb *metadata.ResourceBuilder) *metadata.ResourceBuilder {
	rb.SetOracledbInstanceName(s.instanceName)
	rb.SetHostName(s.hostName)
	rb.SetServiceInstanceID(s.serviceInstanceID)
	return rb
}

func getInstanceID(instanceString string, logger *zap.Logger) string {
	hostAndPort, service, found := strings.Cut(instanceString, "/")
	if !found {
		logger.Info("No service name found in the connection string", zap.String("instanceString", instanceString))
	}

	host, port, err := net.SplitHostPort(hostAndPort)
	if err != nil {
		logger.Warn("Computing service.instance.id failed. Couldn't extract host and port from the connection data.", zap.Error(err))
		return constructInstanceID("unknown", "1521", service)
	}

	// Replace the host value with machine name if connecting to localhost target
	if strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback() {
		localhost, err := os.Hostname()
		if err != nil {
			logger.Warn("Failed getting localhost machine name for the service.instance.id.")
		} else {
			host = localhost
		}
	}

	return constructInstanceID(host, port, service)
}

func constructInstanceID(host, port, service string) string {
	if strings.TrimSpace(host) == "" {
		host = "unknown"
	}
	if strings.TrimSpace(port) == "" {
		port = "1521"
	}

	if service != "" {
		return fmt.Sprintf("%s:%s/%s", host, port, service)
	}
	return fmt.Sprintf("%s:%s", host, port)
}

func (s *oracleScraper) calculateLookbackSeconds() int {
	if s.lastExecutionTimestamp.IsZero() {
		return int(s.topQueryCollectCfg.CollectionInterval.Seconds())
	}

	// vsqlRefreshLag is the buffer to account for v$sql maximum refresh latency (5 seconds) + 5 seconds to offset any collection delays.
	// PS: https://docs.oracle.com/en/database/oracle/oracle-database/21/refrn/V-SQL.html
	const vsqlRefreshLag = 10 * time.Second

	return int(math.Ceil(time.Now().
		Add(vsqlRefreshLag).
		Sub(s.lastExecutionTimestamp).Seconds()))
}
