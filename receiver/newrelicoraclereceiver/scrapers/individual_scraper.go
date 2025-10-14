package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	commonutils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/common-utils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// IndividualQueriesScraper contains the scraper for individual queries metrics
type IndividualQueriesScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewIndividualQueriesScraper creates a new Individual Queries Scraper instance
func NewIndividualQueriesScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *IndividualQueriesScraper {
	if db == nil {
		panic("database connection cannot be nil")
	}
	if mb == nil {
		panic("metrics builder cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	if instanceName == "" {
		panic("instance name cannot be empty")
	}

	return &IndividualQueriesScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// ScrapeIndividualQueries collects Oracle individual queries metrics filtered by query IDs
func (s *IndividualQueriesScraper) ScrapeIndividualQueries(ctx context.Context, queryIDs []string) []error {
	s.logger.Info("=== INDIVIDUAL QUERIES SCRAPER CALLED ===",
		zap.Int("total_query_ids", len(queryIDs)),
		zap.Strings("received_query_ids", queryIDs))

	s.logger.Debug("Begin Oracle individual queries scrape", zap.Int("filter_query_ids", len(queryIDs)))

	var scrapeErrors []error

	// If no query IDs to filter, return early
	if len(queryIDs) == 0 {
		s.logger.Warn("No query IDs to filter individual queries, skipping - check slow queries scraper")
		return scrapeErrors
	}

	s.logger.Info("Individual queries scraper starting",
		zap.Int("query_ids_count", len(queryIDs)),
		zap.Strings("query_ids", queryIDs))

	// Build quoted query IDs for the IN clause
	quotedQueryIDs := make([]string, len(queryIDs))
	for i, qid := range queryIDs {
		quotedQueryIDs[i] = fmt.Sprintf("'%s'", qid)
	}

	// Create the filtered SQL query by directly inserting the query IDs
	filteredSQL := fmt.Sprintf(queries.IndividualQueriesFilteredSQL, strings.Join(quotedQueryIDs, ","))

	s.logger.Debug("Executing individual queries with filter", zap.String("sql", filteredSQL))

	// Execute the individual queries SQL with query ID filter
	rows, err := s.db.QueryContext(ctx, filteredSQL)
	if err != nil {
		s.logger.Error("Failed to execute individual queries query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	now := pcommon.NewTimestampFromTime(time.Now())

	rowCount := 0
	for rows.Next() {
		rowCount++
		var individualQuery models.IndividualQuery

		if err := rows.Scan(
			&individualQuery.QueryID,
			&individualQuery.UserID,
			&individualQuery.Username,
			&individualQuery.QueryText,
			&individualQuery.CPUTimeMs,
			&individualQuery.ElapsedTimeMs,
			&individualQuery.Hostname,
			&individualQuery.DatabaseName,
		); err != nil {
			s.logger.Error("Failed to scan individual query row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		// Ensure we have valid values for key fields
		if !individualQuery.IsValidForMetrics() {
			s.logger.Debug("Skipping individual query with null key values",
				zap.String("query_id", individualQuery.GetQueryID()),
				zap.Float64("elapsed_time_ms", individualQuery.ElapsedTimeMs.Float64))
			continue
		}

		// Convert NullString/NullFloat64/NullInt64 to string values for attributes
		qID := individualQuery.GetQueryID()
		qText := commonutils.AnonymizeAndNormalize(individualQuery.GetQueryText())
		userID := individualQuery.GetUserID()
		username := individualQuery.GetUsername()
		hostname := individualQuery.GetHostname()
		dbName := individualQuery.GetDatabaseName()

		s.logger.Debug("Processing individual query",
			zap.String("query_id", qID),
			zap.String("user_id", userID),
			zap.String("username", username),
			zap.String("hostname", hostname),
			zap.Float64("cpu_time_ms", individualQuery.CPUTimeMs.Float64),
			zap.Float64("elapsed_time_ms", individualQuery.ElapsedTimeMs.Float64))

		// Record CPU time if available
		if individualQuery.CPUTimeMs.Valid {
			s.mb.RecordNewrelicoracledbIndividualQueriesCPUTimeDataPoint(
				now,
				individualQuery.CPUTimeMs.Float64,
				dbName,
				qID,
			)
		}

		// Record elapsed time
		s.mb.RecordNewrelicoracledbIndividualQueriesElapsedTimeDataPoint(
			now,
			individualQuery.ElapsedTimeMs.Float64,
			dbName,
			qID,
		)

		// Record query details (count = 1 for each query) with user information
		s.mb.RecordNewrelicoracledbIndividualQueriesQueryDetailsDataPoint(
			now,
			1, // Count of 1 for each query
			qID,
			qText,
			dbName,
			userID,
			username,
			hostname,
		)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over individual queries rows", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}
	s.logger.Info("Completed Oracle individual queries scrape",
		zap.Int("rows_processed", rowCount),
		zap.Int("error_count", len(scrapeErrors)))
	return scrapeErrors
}
