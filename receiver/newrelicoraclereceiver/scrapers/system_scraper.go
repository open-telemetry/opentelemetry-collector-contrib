// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// SystemScraper contains the scraper for system metrics
type SystemScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewSystemScraper creates a new System Scraper instance
func NewSystemScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *SystemScraper {
	return &SystemScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// ScrapeSystemMetrics collects Oracle system metrics from gv$sysmetric
func (s *SystemScraper) ScrapeSystemMetrics(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle system metrics scrape")

	var scrapeErrors []error

	rows, err := s.db.QueryContext(ctx, queries.SystemSysMetricsSQL)
	if err != nil {
		s.logger.Error("Failed to execute system metrics query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var instID sql.NullString
		var metricName sql.NullString
		var value sql.NullFloat64

		if err := rows.Scan(&instID, &metricName, &value); err != nil {
			s.logger.Error("Failed to scan system metric row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		if !metricName.Valid || !value.Valid {
			s.logger.Debug("Skipping system metric with null values",
				zap.String("metric_name", metricName.String),
				zap.Float64("value", value.Float64))
			continue
		}

		instanceIDStr := ""
		if instID.Valid {
			instanceIDStr = instID.String
		}

		s.logger.Debug("Processing system metric",
			zap.String("metric_name", metricName.String),
			zap.Float64("value", value.Float64),
			zap.String("instance_id", instanceIDStr))

		now := pcommon.NewTimestampFromTime(time.Now())

		// Map Oracle metric names to the recording functions
		switch metricName.String {
		case "Buffer Cache Hit Ratio":
			s.mb.RecordNewrelicoracledbSystemBufferCacheHitRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Memory Sorts Ratio":
			s.mb.RecordNewrelicoracledbSystemMemorySortsRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Redo Allocation Hit Ratio":
			s.mb.RecordNewrelicoracledbSystemRedoAllocationHitRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Transaction Per Sec":
			s.mb.RecordNewrelicoracledbSystemTransactionsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalWritesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Direct Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadsDirectPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Direct Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadsDirectPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Direct Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWritesDirectPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Direct Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalWritesDirectPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Direct Lobs Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalLobsReadsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Direct Lobs Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalLobsReadsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Direct Lobs Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalLobsWritesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Direct Lobs Per Txn":
			s.mb.RecordNewrelicoracledbSystemPhysicalLobsWritesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Redo Generated Per Sec":
			s.mb.RecordNewrelicoracledbSystemRedoGeneratedBytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Redo Generated Per Txn":
			s.mb.RecordNewrelicoracledbSystemRedoGeneratedBytesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Logons Per Txn":
			s.mb.RecordNewrelicoracledbSystemLogonsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Open Cursors Per Sec":
			s.mb.RecordNewrelicoracledbSystemOpenCursorsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Open Cursors Per Txn":
			s.mb.RecordNewrelicoracledbSystemOpenCursorsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Commits Per Sec":
			s.mb.RecordNewrelicoracledbSystemUserCommitsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Commits Percentage":
			s.mb.RecordNewrelicoracledbSystemUserCommitsPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Rollbacks Per Sec":
			s.mb.RecordNewrelicoracledbSystemUserRollbacksPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Rollbacks Percentage":
			s.mb.RecordNewrelicoracledbSystemUserRollbacksPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Calls Per Sec":
			s.mb.RecordNewrelicoracledbSystemUserCallsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Calls Per Txn":
			s.mb.RecordNewrelicoracledbSystemUserCallsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Recursive Calls Per Sec":
			s.mb.RecordNewrelicoracledbSystemRecursiveCallsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Recursive Calls Per Txn":
			s.mb.RecordNewrelicoracledbSystemRecursiveCallsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Logical Reads Per Sec":
			s.mb.RecordNewrelicoracledbSystemLogicalReadsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Logical Reads Per Txn":
			s.mb.RecordNewrelicoracledbSystemLogicalReadsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DBWR Checkpoints Per Sec":
			s.mb.RecordNewrelicoracledbSystemDbwrCheckpointsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Background Checkpoints Per Sec":
			s.mb.RecordNewrelicoracledbSystemBackgroundCheckpointsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Redo Writes Per Sec":
			s.mb.RecordNewrelicoracledbSystemRedoWritesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Redo Writes Per Txn":
			s.mb.RecordNewrelicoracledbSystemRedoWritesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Long Table Scans Per Sec":
			s.mb.RecordNewrelicoracledbSystemLongTableScansPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Long Table Scans Per Txn":
			s.mb.RecordNewrelicoracledbSystemLongTableScansPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Table Scans Per Sec":
			s.mb.RecordNewrelicoracledbSystemTotalTableScansPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Table Scans Per Txn":
			s.mb.RecordNewrelicoracledbSystemTotalTableScansPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Full Index Scans Per Sec":
			s.mb.RecordNewrelicoracledbSystemFullIndexScansPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Full Index Scans Per Txn":
			s.mb.RecordNewrelicoracledbSystemFullIndexScansPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Index Scans Per Sec":
			s.mb.RecordNewrelicoracledbSystemTotalIndexScansPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Index Scans Per Txn":
			s.mb.RecordNewrelicoracledbSystemTotalIndexScansPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Parse Count Per Sec":
			s.mb.RecordNewrelicoracledbSystemTotalParseCountPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Parse Count Per Txn":
			s.mb.RecordNewrelicoracledbSystemTotalParseCountPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Hard Parse Count Per Sec":
			s.mb.RecordNewrelicoracledbSystemHardParseCountPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Hard Parse Count Per Txn":
			s.mb.RecordNewrelicoracledbSystemHardParseCountPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Parse Failure Count Per Sec":
			s.mb.RecordNewrelicoracledbSystemParseFailureCountPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Parse Failure Count Per Txn":
			s.mb.RecordNewrelicoracledbSystemParseFailureCountPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Cursor Cache Hit Ratio":
			s.mb.RecordNewrelicoracledbSystemCursorCacheHitRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Disk Sort Per Sec":
			s.mb.RecordNewrelicoracledbSystemDiskSortPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Disk Sort Per Txn":
			s.mb.RecordNewrelicoracledbSystemDiskSortPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Rows Per Sort":
			s.mb.RecordNewrelicoracledbSystemRowsPerSortDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Soft Parse Ratio":
			s.mb.RecordNewrelicoracledbSystemSoftParseRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Calls Ratio":
			s.mb.RecordNewrelicoracledbSystemUserCallsRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Host CPU Utilization (%)":
			s.mb.RecordNewrelicoracledbSystemHostCPUUtilizationDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Network Traffic Volume Per Sec":
			s.mb.RecordNewrelicoracledbSystemNetworkTrafficVolumePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Timeouts Per Sec":
			s.mb.RecordNewrelicoracledbSystemEnqueueTimeoutsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Timeouts Per Txn":
			s.mb.RecordNewrelicoracledbSystemEnqueueTimeoutsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Waits Per Sec":
			s.mb.RecordNewrelicoracledbSystemEnqueueWaitsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Waits Per Txn":
			s.mb.RecordNewrelicoracledbSystemEnqueueWaitsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Deadlocks Per Sec":
			s.mb.RecordNewrelicoracledbSystemEnqueueDeadlocksPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Deadlocks Per Txn":
			s.mb.RecordNewrelicoracledbSystemEnqueueDeadlocksPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Requests Per Sec":
			s.mb.RecordNewrelicoracledbSystemEnqueueRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Enqueue Requests Per Txn":
			s.mb.RecordNewrelicoracledbSystemEnqueueRequestsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Gets Per Sec":
			s.mb.RecordNewrelicoracledbSystemDbBlockGetsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Gets Per Txn":
			s.mb.RecordNewrelicoracledbSystemDbBlockGetsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Consistent Read Gets Per Sec":
			s.mb.RecordNewrelicoracledbSystemConsistentReadGetsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Changes Per Sec":
			s.mb.RecordNewrelicoracledbSystemDbBlockChangesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Consistent Read Gets Per Txn":
			s.mb.RecordNewrelicoracledbSystemConsistentReadGetsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Changes Per Txn":
			s.mb.RecordNewrelicoracledbSystemDbBlockChangesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Consistent Read Changes Per Sec":
			s.mb.RecordNewrelicoracledbSystemConsistentReadChangesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Consistent Read Changes Per Txn":
			s.mb.RecordNewrelicoracledbSystemConsistentReadChangesPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CPU Usage Per Sec":
			s.mb.RecordNewrelicoracledbSystemCPUUsagePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CPU Usage Per Txn":
			s.mb.RecordNewrelicoracledbSystemCPUUsagePerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CR Blocks Created Per Sec":
			s.mb.RecordNewrelicoracledbSystemCrBlocksCreatedPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CR Blocks Created Per Txn":
			s.mb.RecordNewrelicoracledbSystemCrBlocksCreatedPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CR Undo Records Applied Per Sec":
			s.mb.RecordNewrelicoracledbSystemCrUndoRecordsAppliedPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "CR Undo Records Applied Per Txn":
			s.mb.RecordNewrelicoracledbSystemCrUndoRecordsAppliedPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Rollback UndoRec Applied Per Sec":
			s.mb.RecordNewrelicoracledbSystemUserRollbackUndoRecordsAppliedPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Rollback Undo Records Applied Per Txn":
			s.mb.RecordNewrelicoracledbSystemUserRollbackUndoRecordsAppliedPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Leaf Node Splits Per Sec":
			s.mb.RecordNewrelicoracledbSystemLeafNodeSplitsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Leaf Node Splits Per Txn":
			s.mb.RecordNewrelicoracledbSystemLeafNodeSplitsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Branch Node Splits Per Sec":
			s.mb.RecordNewrelicoracledbSystemBranchNodeSplitsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Branch Node Splits Per Txn":
			s.mb.RecordNewrelicoracledbSystemBranchNodeSplitsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Read Total IO Requests Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadTotalIoRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Read Total Bytes Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadTotalBytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "GC CR Block Received Per Second":
			s.mb.RecordNewrelicoracledbSystemGcCrBlockReceivedPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "GC CR Block Received Per Txn":
			s.mb.RecordNewrelicoracledbSystemGcCrBlockReceivedPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "GC Current Block Received Per Second":
			s.mb.RecordNewrelicoracledbSystemGcCurrentBlockReceivedPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "GC Current Block Received Per Txn":
			s.mb.RecordNewrelicoracledbSystemGcCurrentBlockReceivedPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Global Cache Average CR Get Time":
			s.mb.RecordNewrelicoracledbSystemGlobalCacheAverageCrGetTimeDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Global Cache Average Current Get Time":
			s.mb.RecordNewrelicoracledbSystemGlobalCacheAverageCurrentGetTimeDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Write Total IO Requests Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWriteTotalIoRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Global Cache Blocks Corrupted":
			s.mb.RecordNewrelicoracledbSystemGlobalCacheBlocksCorruptedDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Global Cache Blocks Lost":
			s.mb.RecordNewrelicoracledbSystemGlobalCacheBlocksLostDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Current Logons Count":
			s.mb.RecordNewrelicoracledbSystemCurrentLogonsCountDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Current Open Cursors Count":
			s.mb.RecordNewrelicoracledbSystemCurrentOpenCursorsCountDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "User Limit %":
			s.mb.RecordNewrelicoracledbSystemUserLimitPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "SQL Service Response Time":
			s.mb.RecordNewrelicoracledbSystemSQLServiceResponseTimeDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Database Wait Time Ratio":
			s.mb.RecordNewrelicoracledbSystemDatabaseWaitTimeRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Database CPU Time Ratio":
			s.mb.RecordNewrelicoracledbSystemDatabaseCPUTimeRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Response Time Per Txn":
			s.mb.RecordNewrelicoracledbSystemResponseTimePerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Row Cache Hit Ratio":
			s.mb.RecordNewrelicoracledbSystemRowCacheHitRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Row Cache Miss Ratio":
			s.mb.RecordNewrelicoracledbSystemRowCacheMissRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Library Cache Hit Ratio":
			s.mb.RecordNewrelicoracledbSystemLibraryCacheHitRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Library Cache Miss Ratio":
			s.mb.RecordNewrelicoracledbSystemLibraryCacheMissRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Shared Pool Free %":
			s.mb.RecordNewrelicoracledbSystemSharedPoolFreePercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "PGA Cache Hit %":
			s.mb.RecordNewrelicoracledbSystemPgaCacheHitPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Process Limit %":
			s.mb.RecordNewrelicoracledbSystemProcessLimitPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Session Limit %":
			s.mb.RecordNewrelicoracledbSystemSessionLimitPercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Executions Per Txn":
			s.mb.RecordNewrelicoracledbSystemExecutionsPerTransactionDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Executions Per Sec":
			s.mb.RecordNewrelicoracledbSystemExecutionsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Txns Per Logon":
			s.mb.RecordNewrelicoracledbSystemTransactionsPerLogonDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Database Time Per Sec":
			s.mb.RecordNewrelicoracledbSystemDatabaseTimePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Write Total Bytes Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWriteTotalBytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Write IO Requests Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWriteIoRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Changes Per User Call":
			s.mb.RecordNewrelicoracledbSystemDbBlockChangesPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "DB Block Gets Per User Call":
			s.mb.RecordNewrelicoracledbSystemDbBlockGetsPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Executions Per User Call":
			s.mb.RecordNewrelicoracledbSystemExecutionsPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Logical Reads Per User Call":
			s.mb.RecordNewrelicoracledbSystemLogicalReadsPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Sorts Per User Call":
			s.mb.RecordNewrelicoracledbSystemTotalSortsPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Total Table Scans Per User Call":
			s.mb.RecordNewrelicoracledbSystemTotalTableScansPerUserCallDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Current OS Load":
			s.mb.RecordNewrelicoracledbSystemCurrentOsLoadDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Streams Pool Usage Percentage":
			s.mb.RecordNewrelicoracledbSystemStreamsPoolUsagePercentageDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "I/O Megabytes per Second":
			s.mb.RecordNewrelicoracledbSystemIoMegabytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "I/O Requests per Second":
			s.mb.RecordNewrelicoracledbSystemIoRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Average Active Sessions":
			s.mb.RecordNewrelicoracledbSystemAverageActiveSessionsDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Active Serial Sessions":
			s.mb.RecordNewrelicoracledbSystemActiveSerialSessionsDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Active Parallel Sessions":
			s.mb.RecordNewrelicoracledbSystemActiveParallelSessionsDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Background CPU Usage Per Sec":
			s.mb.RecordNewrelicoracledbSystemBackgroundCPUUsagePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Background Time Per Sec":
			s.mb.RecordNewrelicoracledbSystemBackgroundTimePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Host CPU Usage Per Sec":
			s.mb.RecordNewrelicoracledbSystemHostCPUUsagePerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Temp Space Used":
			s.mb.RecordNewrelicoracledbSystemTempSpaceUsedDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Session Count":
			s.mb.RecordNewrelicoracledbSystemSessionCountDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Captured user calls":
			s.mb.RecordNewrelicoracledbSystemCapturedUserCallsDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Execute Without Parse Ratio":
			s.mb.RecordNewrelicoracledbSystemExecuteWithoutParseRatioDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Logons Per Sec":
			s.mb.RecordNewrelicoracledbSystemLogonsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Read Bytes Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadBytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Read IO Requests Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadIoRequestsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Reads Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalReadsPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Write Bytes Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWriteBytesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		case "Physical Writes Per Sec":
			s.mb.RecordNewrelicoracledbSystemPhysicalWritesPerSecondDataPoint(now, value.Float64, s.instanceName, instanceIDStr)
		default:
			s.logger.Debug("Unknown system metric", zap.String("metric_name", metricName.String))
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over system metrics rows", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}

	s.logger.Debug("Completed Oracle system metrics scrape", zap.Int("errors", len(scrapeErrors)))
	return scrapeErrors
}
