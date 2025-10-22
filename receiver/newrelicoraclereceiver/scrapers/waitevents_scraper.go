package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// WaitEventsScraper contains the scraper for wait events metrics
type WaitEventsScraper struct {
	db                            *sql.DB
	mb                            *metadata.MetricsBuilder
	logger                        *zap.Logger
	instanceName                  string
	metricsBuilderConfig          metadata.MetricsBuilderConfig
	queryMonitoringCountThreshold int
}

// NewWaitEventsScraper creates a new Wait Events Scraper instance
func NewWaitEventsScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig, countThreshold int) (*WaitEventsScraper, error) {
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

	return &WaitEventsScraper{
		db:                            db,
		mb:                            mb,
		logger:                        logger,
		instanceName:                  instanceName,
		metricsBuilderConfig:          metricsBuilderConfig,
		queryMonitoringCountThreshold: countThreshold,
	}, nil
}

// ScrapeWaitEvents collects Oracle wait events metrics
func (s *WaitEventsScraper) ScrapeWaitEvents(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle wait events scrape")

	var scrapeErrors []error

	// Execute the wait events SQL with configured threshold using parameterized query
	waitEventQueriesSQL, params := queries.GetWaitEventQueriesSQL(s.queryMonitoringCountThreshold)
	rows, err := s.db.QueryContext(ctx, waitEventQueriesSQL, params...)
	if err != nil {
		s.logger.Error("Failed to execute wait events query", zap.Error(err))
		return []error{err}
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close wait events result set", zap.Error(closeErr))
		}
	}()

	now := pcommon.NewTimestampFromTime(time.Now())
	rowCount := 0

	for rows.Next() {
		rowCount++
		var waitEvent models.WaitEvent

		if err := rows.Scan(
			&waitEvent.DatabaseName,
			&waitEvent.QueryID,
			&waitEvent.WaitCategory,
			&waitEvent.WaitEventName,
			&waitEvent.CollectionTimestamp,
			&waitEvent.WaitingTasksCount,
			&waitEvent.TotalWaitTimeMs,
			&waitEvent.AvgWaitTimeMs,
		); err != nil {
			s.logger.Error("Failed to scan wait events row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		// Ensure we have valid values for key fields
		if !waitEvent.IsValidForMetrics() {
			s.logger.Debug("Skipping wait event with null key values",
				zap.String("query_id", waitEvent.GetQueryID()),
				zap.String("wait_event_name", waitEvent.GetWaitEventName()),
				zap.Float64("total_wait_time_ms", waitEvent.GetTotalWaitTimeMs()))
			continue
		}

		// Convert NullString/NullInt64/NullFloat64 to string values for attributes
		dbName := waitEvent.GetDatabaseName()
		qID := waitEvent.GetQueryID()
		waitCat := waitEvent.GetWaitCategory()
		waitEventName := waitEvent.GetWaitEventName()

		s.logger.Debug("Processing wait event",
			zap.String("database_name", dbName),
			zap.String("query_id", qID),
			zap.String("wait_category", waitCat),
			zap.String("wait_event_name", waitEventName),
			zap.Float64("total_wait_time_ms", waitEvent.GetTotalWaitTimeMs()),
			zap.Float64("avg_wait_time_ms", waitEvent.GetAvgWaitTimeMs()))

		// Record waiting tasks count if available
		if waitEvent.HasValidWaitingTasksCount() {
			s.mb.RecordNewrelicoracledbWaitEventsWaitingTasksCountDataPoint(
				now,
				float64(waitEvent.GetWaitingTasksCount()),
				dbName,
				qID,
				waitEventName,
				waitCat,
			)
		}

		// Record total wait time
		s.mb.RecordNewrelicoracledbWaitEventsTotalWaitTimeMsDataPoint(
			now,
			waitEvent.GetTotalWaitTimeMs(),
			dbName,
			qID,
			waitEventName,
			waitCat,
		)

		// Record average wait time if available
		if waitEvent.HasValidAvgWaitTime() {
			s.mb.RecordNewrelicoracledbWaitEventsAvgWaitTimeMsDataPoint(
				now,
				waitEvent.GetAvgWaitTimeMs(),
				dbName,
				qID,
				waitEventName,
				waitCat,
			)
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over wait events rows", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}

	s.logger.Debug("Completed Oracle wait events scrape",
		zap.Int("rows_processed", rowCount),
		zap.Int("errors_encountered", len(scrapeErrors)))
	return scrapeErrors
}
