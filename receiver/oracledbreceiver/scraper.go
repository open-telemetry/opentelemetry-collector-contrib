// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	samplesQuery = `
	    SELECT /* collector-query */  S.MACHINE, S.USERNAME, S.SCHEMANAME, S.SQL_ID, 
		S.SQL_CHILD_NUMBER, S.SID, S.SERIAL#, Q.SQL_FULLTEXT, S.OSUSER, S.PROCESS, 
		S.PORT, S.PROGRAM, S.MODULE, S.STATUS, S.STATE, Q.PLAN_HASH_VALUE, 
		ROUND((SYSDATE - SQL_EXEC_START) * 86400) AS DURATION_SEC, 
		CASE WHEN S.TIME_REMAINING_MICRO IS NOT NULL 
		      THEN S.WAIT_CLASS END AS WAIT_CLASS, 
		CASE WHEN S.TIME_REMAINING_MICRO IS NOT NULL 
		      THEN S.EVENT END AS EVENT, 
		CASE WHEN S.PLSQL_ENTRY_OBJECT_ID IS NOT NULL 
		      THEN CASE WHEN P.PROCEDURE_NAME IS NULL 
			             THEN P.OWNER || '.' || P.OBJECT_NAME ELSE P.OWNER || '.' || P.OBJECT_NAME || '.' || P.PROCEDURE_NAME END 
			  END AS OBJECT_NAME, P.OBJECT_TYPE 
		FROM V$SESSION S LEFT JOIN DBA_PROCEDURES P ON S.PLSQL_ENTRY_OBJECT_ID = P.OBJECT_ID 
		      AND S.PLSQL_ENTRY_SUBPROGRAM_ID = P.SUBPROGRAM_ID 
			  LEFT JOIN V$SQL Q ON S.SQL_ID = Q.SQL_ID 
			  WHERE S.SQL_ID IS NOT NULL AND S.STATUS = 'ACTIVE'`
)

type dbProviderFunc func() (*sql.DB, error)

type clientProviderFunc func(*sql.DB, string, *zap.Logger) dbClient

type oracleScraper struct {
	statsClient                dbClient
	tablespaceUsageClient      dbClient
	systemResourceLimitsClient dbClient
	sessionCountClient         dbClient
	samplesQueryClient         dbClient
	db                         *sql.DB
	clientProviderFunc         clientProviderFunc
	mb                         *metadata.MetricsBuilder
	dbProviderFunc             dbProviderFunc
	logger                     *zap.Logger
	id                         component.ID
	instanceName               string
	scrapeCfg                  scraperhelper.ControllerConfig
	startTime                  pcommon.Timestamp
	metricsBuilderConfig       metadata.MetricsBuilderConfig
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

func newLogsScraper(scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string) (scraper.Logs, error) {
	s := &oracleScraper{
		scrapeCfg:          scrapeCfg,
		logger:             logger,
		dbProviderFunc:     providerFunc,
		clientProviderFunc: clientProviderFunc,
		instanceName:       instanceName,
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

func (s *oracleScraper) scrapeLogs(ctx context.Context) (plog.Logs, error) {
	const dbPrefix = "oracledb."
	const queryPrefix = "query."
	const duration = "DURATION_SEC"
	const event = "EVENT"
	const hostName = "MACHINE"
	const module = "MODULE"
	const osUser = "OSUSER"
	const objectName = "OBJECT_NAME"
	const objectType = "OBJECT_TYPE"
	const process = "PROCESS"
	const port = "PORT"
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

	logs := plog.NewLogs()

	resourceLog := logs.ResourceLogs().AppendEmpty()
	resourceAttributes := resourceLog.Resource().Attributes()

	resourceAttributes.PutStr(dbPrefix+"instance.name", s.instanceName)

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	for _, row := range rows {

		queryPlanHashVal := hex.EncodeToString([]byte(row[planHashValue]))
		record := scopedLog.LogRecords().AppendEmpty()
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

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

		obfuscatedSQL, err := ObfuscateSQL(row[sqlText])

		if err != nil {
			s.logger.Error(fmt.Sprintf("oracleScraper failed getting metric row: %s", err))
		} else {
			record.Attributes().PutStr(strings.ToLower("db.query.text"), obfuscatedSQL)
		}

		record.Attributes().PutStr(strings.ToLower(dbPrefix+queryPrefix+osUser), row[osUser])

		i, err := strconv.ParseInt(row[duration], 10, 64)

		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse int64 for Duration, value was %s: %w", row[duration], err))

		}
		record.Attributes().PutInt(dbPrefix+queryPrefix+"duration", i)
	}

	return logs, errors.Join(scrapeErrors...)

}

func (s *oracleScraper) shutdown(_ context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
