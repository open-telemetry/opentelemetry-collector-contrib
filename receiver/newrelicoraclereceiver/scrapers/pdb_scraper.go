// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// PdbScraper handles Oracle PDB sys metrics
type PdbScraper struct {
	db           *sql.DB
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewPdbScraper creates a new PDB scraper
func NewPdbScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *PdbScraper {
	return &PdbScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapePdbMetrics collects Oracle PDB sys metrics
func (s *PdbScraper) ScrapePdbMetrics(ctx context.Context) []error {
	var errors []error

	s.logger.Debug("Scraping Oracle PDB sys metrics")
	now := pcommon.NewTimestampFromTime(time.Now())

	// Scrape PDB sys metrics
	errors = append(errors, s.scrapePDBSysMetrics(ctx, now)...)

	return errors
}

// scrapePDBSysMetrics scrapes PDB sys metrics from gv$con_sysmetric view
func (s *PdbScraper) scrapePDBSysMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	rows, err := s.db.QueryContext(ctx, queries.PDBSysMetricsSQL)
	if err != nil {
		s.logger.Error("Failed to execute PDB sys metrics query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var instID int
		var metricName string
		var value float64

		if err := rows.Scan(&instID, &metricName, &value); err != nil {
			s.logger.Error("Error scanning PDB sys metrics row", zap.Error(err))
			continue
		}

		instanceIDStr := strconv.Itoa(instID)

		// Map metric names to recording functions based on the identifiers
		switch metricName {
		case "Active Parallel Sessions":
			s.mb.RecordNewrelicoracledbPdbActiveParallelSessionsDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Active Serial Sessions":
			s.mb.RecordNewrelicoracledbPdbActiveSerialSessionsDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Average Active Sessions":
			s.mb.RecordNewrelicoracledbPdbAverageActiveSessionsDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Background CPU Usage Per Sec":
			s.mb.RecordNewrelicoracledbPdbBackgroundCPUUsagePerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Background Time Per Sec":
			s.mb.RecordNewrelicoracledbPdbBackgroundTimePerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "CPU Usage Per Sec":
			s.mb.RecordNewrelicoracledbPdbCPUUsagePerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "CPU Usage Per Txn":
			s.mb.RecordNewrelicoracledbPdbCPUUsagePerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Current Logons Count":
			s.mb.RecordNewrelicoracledbPdbCurrentLogonsDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Current Open Cursors Count":
			s.mb.RecordNewrelicoracledbPdbCurrentOpenCursorsDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Database CPU Time Ratio":
			s.mb.RecordNewrelicoracledbPdbCPUTimeRatioDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Database Wait Time Ratio":
			s.mb.RecordNewrelicoracledbPdbWaitTimeRatioDataPoint(now, value, s.instanceName, instanceIDStr)
		case "DB Block Changes Per Sec":
			s.mb.RecordNewrelicoracledbPdbBlockChangesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "DB Block Changes Per Txn":
			s.mb.RecordNewrelicoracledbPdbBlockChangesPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Executions Per Sec":
			s.mb.RecordNewrelicoracledbPdbExecutionsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Executions Per Txn":
			s.mb.RecordNewrelicoracledbPdbExecutionsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Hard Parse Count Per Sec":
			s.mb.RecordNewrelicoracledbPdbHardParseCountPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Hard Parse Count Per Txn":
			s.mb.RecordNewrelicoracledbPdbHardParseCountPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Logical Reads Per Sec":
			s.mb.RecordNewrelicoracledbPdbLogicalReadsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Logical Reads Per Txn":
			s.mb.RecordNewrelicoracledbPdbLogicalReadsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Logons Per Txn":
			s.mb.RecordNewrelicoracledbPdbLogonsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Network Traffic Volume Per Sec":
			s.mb.RecordNewrelicoracledbPdbNetworkTrafficBytePerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Open Cursors Per Sec":
			s.mb.RecordNewrelicoracledbPdbOpenCursorsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Open Cursors Per Txn":
			s.mb.RecordNewrelicoracledbPdbOpenCursorsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Parse Failure Count Per Sec":
			s.mb.RecordNewrelicoracledbPdbParseFailureCountPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Read Total Bytes Per Sec":
			s.mb.RecordNewrelicoracledbPdbPhysicalReadBytesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Reads Per Txn":
			s.mb.RecordNewrelicoracledbPdbPhysicalReadsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Write Total Bytes Per Sec":
			s.mb.RecordNewrelicoracledbPdbPhysicalWriteBytesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Writes Per Txn":
			s.mb.RecordNewrelicoracledbPdbPhysicalWritesPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Redo Generated Per Sec":
			s.mb.RecordNewrelicoracledbPdbRedoGeneratedBytesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Redo Generated Per Txn":
			s.mb.RecordNewrelicoracledbPdbRedoGeneratedBytesPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Response Time Per Txn":
			s.mb.RecordNewrelicoracledbPdbResponseTimePerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Session Count":
			s.mb.RecordNewrelicoracledbPdbSessionCountDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Soft Parse Ratio":
			s.mb.RecordNewrelicoracledbPdbSoftParseRatioDataPoint(now, value, s.instanceName, instanceIDStr)
		case "SQL Service Response Time":
			s.mb.RecordNewrelicoracledbPdbSQLServiceResponseTimeDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Total Parse Count Per Sec":
			s.mb.RecordNewrelicoracledbPdbTotalParseCountPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Total Parse Count Per Txn":
			s.mb.RecordNewrelicoracledbPdbTotalParseCountPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Calls Per Sec":
			s.mb.RecordNewrelicoracledbPdbUserCallsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Calls Per Txn":
			s.mb.RecordNewrelicoracledbPdbUserCallsPerTransactionDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Commits Per Sec":
			s.mb.RecordNewrelicoracledbPdbUserCommitsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Commits Percentage":
			s.mb.RecordNewrelicoracledbPdbUserCommitsPercentageDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Rollbacks Per Sec":
			s.mb.RecordNewrelicoracledbPdbUserRollbacksPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Rollbacks Percentage":
			s.mb.RecordNewrelicoracledbPdbUserRollbacksPercentageDataPoint(now, value, s.instanceName, instanceIDStr)
		case "User Transaction Per Sec":
			s.mb.RecordNewrelicoracledbPdbTransactionsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Execute Without Parse Ratio":
			s.mb.RecordNewrelicoracledbPdbExecuteWithoutParseRatioDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Logons Per Sec":
			s.mb.RecordNewrelicoracledbPdbLogonsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Read Bytes Per Sec":
			s.mb.RecordNewrelicoracledbPdbDbPhysicalReadBytesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Reads Per Sec":
			s.mb.RecordNewrelicoracledbPdbDbPhysicalReadsPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Write Bytes Per Sec":
			s.mb.RecordNewrelicoracledbPdbDbPhysicalWriteBytesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		case "Physical Writes Per Sec":
			s.mb.RecordNewrelicoracledbPdbDbPhysicalWritesPerSecondDataPoint(now, value, s.instanceName, instanceIDStr)
		default:
			// Log unknown metric name but don't error
			s.logger.Debug("Unknown PDB sys metric name", zap.String("metric_name", metricName))
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over PDB sys metrics rows", zap.Error(err))
		return []error{err}
	}

	return nil
}
