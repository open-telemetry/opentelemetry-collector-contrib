// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	metricsBuilder             *metadata.MetricsBuilder
	dbProviderFunc             dbProviderFunc
	logger                     *zap.Logger
	id                         component.ID
	instanceName               string
	scrapeCfg                  scraperhelper.ScraperControllerSettings
	startTime                  pcommon.Timestamp
	metricsSettings            metadata.MetricsSettings
}

func newScraper(id component.ID, metricsBuilder *metadata.MetricsBuilder, metricsSettings metadata.MetricsSettings, scrapeCfg scraperhelper.ScraperControllerSettings, logger *zap.Logger, providerFunc dbProviderFunc, clientProviderFunc clientProviderFunc, instanceName string) *scraper {
	return &scraper{
		id:                 id,
		metricsBuilder:     metricsBuilder,
		metricsSettings:    metricsSettings,
		scrapeCfg:          scrapeCfg,
		logger:             logger,
		dbProviderFunc:     providerFunc,
		clientProviderFunc: clientProviderFunc,
		instanceName:       instanceName,
	}
}

var _ scraperhelper.Scraper = (*scraper)(nil)

func (s *scraper) ID() component.ID {
	return s.id
}

func (s *scraper) Start(context.Context, component.Host) error {
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

func (s *scraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Begin scrape")

	var scrapeErrors []error

	runStats := s.metricsSettings.OracledbEnqueueDeadlocks.Enabled || s.metricsSettings.OracledbExchangeDeadlocks.Enabled || s.metricsSettings.OracledbExecutions.Enabled || s.metricsSettings.OracledbParseCalls.Enabled || s.metricsSettings.OracledbHardParses.Enabled || s.metricsSettings.OracledbUserCommits.Enabled || s.metricsSettings.OracledbUserRollbacks.Enabled || s.metricsSettings.OracledbPhysicalReads.Enabled || s.metricsSettings.OracledbLogicalReads.Enabled || s.metricsSettings.OracledbCPUTime.Enabled || s.metricsSettings.OracledbPgaMemory.Enabled
	if runStats {
		now := pcommon.NewTimestampFromTime(time.Now())
		rows, execError := s.statsClient.metricRows(ctx)
		if execError != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", statsSQL, execError))
		}

		for _, row := range rows {
			switch row["NAME"] {
			case enqueueDeadlocks:
				if s.metricsSettings.OracledbEnqueueDeadlocks.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", enqueueDeadlocks, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbEnqueueDeadlocksDataPoint(now, value)
				}
			case exchangeDeadlocks:
				if s.metricsSettings.OracledbExchangeDeadlocks.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", exchangeDeadlocks, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbExchangeDeadlocksDataPoint(now, value)
				}
			case executeCount:
				if s.metricsSettings.OracledbExecutions.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", executeCount, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbExecutionsDataPoint(now, value)
				}
			case parseCountTotal:
				if s.metricsSettings.OracledbParseCalls.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", parseCountTotal, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbParseCallsDataPoint(now, value)
				}
			case parseCountHard:
				if s.metricsSettings.OracledbHardParses.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", parseCountHard, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbHardParsesDataPoint(now, value)
				}
			case userCommits:
				if s.metricsSettings.OracledbUserCommits.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", userCommits, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbUserCommitsDataPoint(now, value)
				}
			case userRollbacks:
				if s.metricsSettings.OracledbUserRollbacks.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", userRollbacks, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbUserRollbacksDataPoint(now, value)
				}
			case physicalReads:
				if s.metricsSettings.OracledbPhysicalReads.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", physicalReads, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbPhysicalReadsDataPoint(now, value)
				}
			case sessionLogicalReads:
				if s.metricsSettings.OracledbLogicalReads.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", sessionLogicalReads, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbLogicalReadsDataPoint(now, value)
				}
			case cpuTime:
				if s.metricsSettings.OracledbCPUTime.Enabled {
					value, err := strconv.ParseFloat(row["VALUE"], 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", cpuTime, row["VALUE"], err))
					}
					// divide by 100 as the value is expressed in tens of milliseconds
					value /= 100
					s.metricsBuilder.RecordOracledbCPUTimeDataPoint(now, value)
				}
			case pgaMemory:
				if s.metricsSettings.OracledbPgaMemory.Enabled {
					value, err := strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("%s value: %q, %w", pgaMemory, row["VALUE"], err))
					}
					s.metricsBuilder.RecordOracledbPgaMemoryDataPoint(pcommon.NewTimestampFromTime(time.Now()), value)
				}
			}
		}
	}

	if s.metricsSettings.OracledbSessionsUsage.Enabled {
		rows, err := s.sessionCountClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", sessionCountSQL, err))
		}
		for _, row := range rows {
			value, err := strconv.ParseInt(row["VALUE"], 10, 64)
			if err != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("value: %q: %q, %w", row["VALUE"], sessionCountSQL, err))
			}
			s.metricsBuilder.RecordOracledbSessionsUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), value, row["TYPE"], row["STATUS"])
		}
	}

	if s.metricsSettings.OracledbSessionsLimit.Enabled ||
		s.metricsSettings.OracledbProcessesUsage.Enabled || s.metricsSettings.OracledbProcessesLimit.Enabled ||
		s.metricsSettings.OracledbEnqueueResourcesUsage.Enabled || s.metricsSettings.OracledbEnqueueResourcesLimit.Enabled ||
		s.metricsSettings.OracledbEnqueueLocksLimit.Enabled || s.metricsSettings.OracledbEnqueueLocksUsage.Enabled {
		rows, err := s.systemResourceLimitsClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", systemResourceLimitsSQL, err))
		}
		for _, row := range rows {
			resourceName := row["RESOURCE_NAME"]
			switch resourceName {
			case "processes":
				if s.metricsSettings.OracledbProcessesUsage.Enabled {
					currentUsage, err := strconv.ParseInt(row["CURRENT_UTILIZATION"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("current Usage for %q: %q, %s, %w", resourceName, row["CURRENT_Usage"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbProcessesUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), currentUsage)
					}
				}
				if s.metricsSettings.OracledbProcessesLimit.Enabled {
					maxUsage, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max Usage for %q: %q, %s, %w", resourceName, row["MAX_Usage"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbProcessesLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUsage)
					}
				}
			case "sessions":
				if s.metricsSettings.OracledbSessionsLimit.Enabled {
					maxUtilization, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max utilization for %q: %q, %s, %w", resourceName, row["LIMIT_VALUE"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbSessionsLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUtilization)
					}
				}
			case "enqueue_locks":
				if s.metricsSettings.OracledbEnqueueLocksUsage.Enabled {
					currentUtilization, err := strconv.ParseInt(row["CURRENT_UTILIZATION"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("current utilization for %q: %q, %s, %w", resourceName, row["CURRENT_UTILIZATION"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbEnqueueLocksUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), currentUtilization)
					}
				}
				if s.metricsSettings.OracledbEnqueueLocksLimit.Enabled {
					maxUtilization, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max utilization for %q: %q, %s, %w", resourceName, row["LIMIT_VALUE"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbEnqueueLocksLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUtilization)
					}
				}
			case "dml_locks":
				if s.metricsSettings.OracledbDmlLocksUsage.Enabled {
					currentUtilization, err := strconv.ParseInt(row["CURRENT_UTILIZATION"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("current utilization for %q: %q, %s, %w", resourceName, row["CURRENT_UTILIZATION"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbDmlLocksUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), currentUtilization)
					}
				}
				if s.metricsSettings.OracledbDmlLocksLimit.Enabled {
					maxUtilization, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max utilization for %q: %q, %s, %w", resourceName, row["LIMIT_VALUE"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbDmlLocksLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUtilization)
					}
				}
			case "enqueue_resources":
				if s.metricsSettings.OracledbEnqueueResourcesUsage.Enabled {
					currentUtilization, err := strconv.ParseInt(row["CURRENT_UTILIZATION"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("current utilization for %q: %q, %s, %w", resourceName, row["CURRENT_UTILIZATION"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbEnqueueResourcesUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), currentUtilization)
					}
				}
				if s.metricsSettings.OracledbEnqueueResourcesLimit.Enabled {
					maxUtilization, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max utilization for %q: %q, %s, %w", resourceName, row["LIMIT_VALUE"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbEnqueueResourcesLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUtilization)
					}
				}
			case "transactions":
				if s.metricsSettings.OracledbTransactionsUsage.Enabled {
					currentUtilization, err := strconv.ParseInt(row["CURRENT_UTILIZATION"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("current utilization for %q: %q, %s, %w", resourceName, row["CURRENT_UTILIZATION"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbTransactionsUsageDataPoint(pcommon.NewTimestampFromTime(time.Now()), currentUtilization)
					}
				}
				if s.metricsSettings.OracledbTransactionsLimit.Enabled {
					maxUtilization, err := strconv.ParseInt(row["LIMIT_VALUE"], 10, 64)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("max utilization for %q: %q, %s, %w", resourceName, row["LIMIT_VALUE"], systemResourceLimitsSQL, err))
					} else {
						s.metricsBuilder.RecordOracledbTransactionsLimitDataPoint(pcommon.NewTimestampFromTime(time.Now()), maxUtilization)
					}
				}
			}
		}
	}
	if s.metricsSettings.OracledbTablespaceSizeUsage.Enabled {
		rows, err := s.tablespaceUsageClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", tablespaceUsageSQL, err))
		} else {
			now := pcommon.NewTimestampFromTime(time.Now())
			for _, row := range rows {
				tablespaceName := row["TABLESPACE_NAME"]
				value, err := strconv.ParseInt(row["BYTES"], 10, 64)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("bytes for %q: %q, %s, %w", tablespaceName, row["BYTES"], tablespaceUsageSQL, err))
				} else {
					s.metricsBuilder.RecordOracledbTablespaceSizeUsageDataPoint(now, value, tablespaceName)
				}
			}
		}
	}
	if s.metricsSettings.OracledbTablespaceSizeLimit.Enabled {
		rows, err := s.tablespaceMaxSpaceClient.metricRows(ctx)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("error executing %s: %w", tablespaceMaxSpaceSQL, err))
		} else {
			now := pcommon.NewTimestampFromTime(time.Now())
			for _, row := range rows {
				tablespaceName := row["TABLESPACE_NAME"]
				var value int64
				ok := true
				if row["VALUE"] == "" {
					value = 0
				} else {
					value, err = strconv.ParseInt(row["VALUE"], 10, 64)
					if err != nil {
						ok = false
						scrapeErrors = append(scrapeErrors, fmt.Errorf("value for %q: %q, %s, %w", tablespaceName, row["VALUE"], tablespaceMaxSpaceSQL, err))
					}
				}
				if ok {
					s.metricsBuilder.RecordOracledbTablespaceSizeLimitDataPoint(now, value, tablespaceName)
				}
			}
		}
	}

	out := s.metricsBuilder.Emit(metadata.WithOracledbInstanceName(s.instanceName))
	s.logger.Debug("Done scraping")
	if len(scrapeErrors) > 0 {
		return out, scrapererror.NewPartialScrapeError(multierr.Combine(scrapeErrors...), len(scrapeErrors))
	}
	return out, nil
}

func (s *scraper) Shutdown(ctx context.Context) error {
	return s.db.Close()
}
