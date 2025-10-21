package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	commonutils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/common-utils"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// SlowQueriesScraper contains the scraper for slow queries metrics
type SlowQueriesScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewSlowQueriesScraper creates a new Slow Queries Scraper instance
func NewSlowQueriesScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) (*SlowQueriesScraper, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection cannot be nil")
	}
	if mb == nil {
		return nil, fmt.Errorf("metrics builder cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if instanceName == "" {
		return nil, fmt.Errorf("instance name cannot be empty")
	}

	return &SlowQueriesScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}, nil
}

// ScrapeSlowQueries collects Oracle slow queries metrics and returns a list of query IDs
func (s *SlowQueriesScraper) ScrapeSlowQueries(ctx context.Context) ([]string, []error) {
	s.logger.Debug("Begin Oracle slow queries scrape")

	var scrapeErrors []error
	var queryIDs []string

	// Execute the slow queries SQL
	rows, err := s.db.QueryContext(ctx, queries.SlowQueriesSQL)
	if err != nil {
		s.logger.Error("Failed to execute slow queries query", zap.Error(err))
		return nil, []error{err}
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close slow queries result set", zap.Error(closeErr))
		}
	}()

	now := pcommon.NewTimestampFromTime(time.Now())
	rowCount := 0

	for rows.Next() {
		rowCount++
		var slowQuery models.SlowQuery

		if err := rows.Scan(
			&slowQuery.DatabaseName,
			&slowQuery.QueryID,
			&slowQuery.SchemaName,
			&slowQuery.UserName,
			&slowQuery.LastLoadTime,
			&slowQuery.SharableMemoryBytes,
			&slowQuery.PersistentMemoryBytes,
			&slowQuery.RuntimeMemoryBytes,
			&slowQuery.StatementType,
			&slowQuery.ExecutionCount,
			&slowQuery.QueryText,
			&slowQuery.AvgCPUTimeMs,
			&slowQuery.AvgDiskReads,
			&slowQuery.AvgDiskWrites,
			&slowQuery.AvgElapsedTimeMs,
			&slowQuery.HasFullTableScan,
		); err != nil {
			s.logger.Error("Failed to scan slow query row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		// Ensure we have valid values for key fields
		if !slowQuery.IsValidForMetrics() {
			s.logger.Debug("Skipping slow query with null key values",
				zap.String("query_id", slowQuery.GetQueryID()),
				zap.Float64("avg_elapsed_time_ms", slowQuery.AvgElapsedTimeMs.Float64))
			continue
		}

		// Convert NullString/NullInt64/NullFloat64 to string values for attributes
		dbName := slowQuery.GetDatabaseName()
		qID := slowQuery.GetQueryID()
		qText := commonutils.AnonymizeAndNormalize(slowQuery.GetQueryText())

		schName := slowQuery.GetSchemaName()
		userName := slowQuery.GetUserName()
		lastLoadTime := slowQuery.GetLastLoadTime()
		stmtType := slowQuery.GetStatementType()
		fullScan := slowQuery.GetHasFullTableScan()

		s.logger.Debug("Processing slow query",
			zap.String("database_name", dbName),
			zap.String("query_id", qID),
			zap.String("schema_name", schName),
			zap.String("user_name", userName),
			zap.String("last_load_time", lastLoadTime),
			zap.String("statement_type", stmtType),
			zap.Int64("sharable_memory_bytes", slowQuery.GetSharableMemoryBytes()),
			zap.Int64("persistent_memory_bytes", slowQuery.GetPersistentMemoryBytes()),
			zap.Int64("runtime_memory_bytes", slowQuery.GetRuntimeMemoryBytes()),
			zap.Float64("avg_elapsed_time_ms", slowQuery.AvgElapsedTimeMs.Float64))

		// Record execution count if available
		if slowQuery.ExecutionCount.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesExecutionCountDataPoint(
				now,
				float64(slowQuery.ExecutionCount.Int64),
				dbName,
				qID,
				userName,
			)
		}

		// Record average CPU time if available
		if slowQuery.AvgCPUTimeMs.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesAvgCPUTimeDataPoint(
				now,
				slowQuery.AvgCPUTimeMs.Float64,
				dbName,
				qID,
				userName,
			)
		}

		// Record average disk reads if available
		if slowQuery.AvgDiskReads.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesAvgDiskReadsDataPoint(
				now,
				slowQuery.AvgDiskReads.Float64,
				dbName,
				qID,
				userName,
			)
		}

		// Record average disk writes if available
		if slowQuery.AvgDiskWrites.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesAvgDiskWritesDataPoint(
				now,
				slowQuery.AvgDiskWrites.Float64,
				dbName,
				qID,
				userName,
			)
		}

		// Record average elapsed time
		s.mb.RecordNewrelicoracledbSlowQueriesAvgElapsedTimeDataPoint(
			now,
			slowQuery.AvgElapsedTimeMs.Float64,
			dbName,
			qID,
			userName,
		)

		// Record memory metrics if available
		if slowQuery.SharableMemoryBytes.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesSharableMemoryDataPoint(
				now,
				slowQuery.SharableMemoryBytes.Int64,
				dbName,
				qID,
				userName,
			)
		}

		if slowQuery.PersistentMemoryBytes.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesPersistentMemoryDataPoint(
				now,
				slowQuery.PersistentMemoryBytes.Int64,
				dbName,
				qID,
				userName,
			)
		}

		if slowQuery.RuntimeMemoryBytes.Valid {
			s.mb.RecordNewrelicoracledbSlowQueriesRuntimeMemoryDataPoint(
				now,
				slowQuery.RuntimeMemoryBytes.Int64,
				dbName,
				qID,
				userName,
			)
		}

		s.mb.RecordNewrelicoracledbSlowQueriesQueryDetailsDataPoint(
			now,
			1,
			dbName,
			qID,
			qText,
			schName,
			stmtType,
			fullScan,
			userName,
		)

		// Add query ID to the list for individual queries processing
		if slowQuery.QueryID.Valid {
			queryIDs = append(queryIDs, slowQuery.QueryID.String)
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over slow queries rows", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}
	s.logger.Debug("Completed Oracle slow queries scrape",
		zap.Int("rows_processed", rowCount),
		zap.Int("query_ids_collected", len(queryIDs)),
		zap.Int("errors_encountered", len(scrapeErrors)))
	return queryIDs, scrapeErrors
}
