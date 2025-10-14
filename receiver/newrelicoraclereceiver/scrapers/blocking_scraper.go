package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	commonutils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/common-utils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// BlockingScraper contains the scraper for blocking queries metrics
type BlockingScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewBlockingScraper creates a new Blocking Queries Scraper instance
func NewBlockingScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *BlockingScraper {
	if db == nil {
		logger.Error("Database connection is nil in NewBlockingScraper")
		return nil
	}

	return &BlockingScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// ScrapeBlockingQueries collects Oracle blocking queries metrics
func (s *BlockingScraper) ScrapeBlockingQueries(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle blocking queries scrape")

	var scrapeErrors []error

	// Execute the blocking queries SQL
	rows, err := s.db.QueryContext(ctx, queries.BlockingQueriesSQL)
	if err != nil {
		s.logger.Error("Failed to execute blocking queries query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	now := pcommon.NewTimestampFromTime(time.Now())

	for rows.Next() {
		var blockingQuery models.BlockingQuery

		if err := rows.Scan(
			&blockingQuery.BlockedSID,
			&blockingQuery.BlockedSerial,
			&blockingQuery.BlockedUser,
			&blockingQuery.BlockedWaitSec,
			&blockingQuery.BlockedSQLID,
			&blockingQuery.BlockedQueryText,
			&blockingQuery.BlockingSID,
			&blockingQuery.BlockingSerial,
			&blockingQuery.BlockingUser,
			&blockingQuery.DatabaseName,
		); err != nil {
			s.logger.Error("Failed to scan blocking query row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		s.logger.Debug("Scraping blocking query",
			zap.String("blocked_user", blockingQuery.GetBlockedUser()),
			zap.String("blocking_user", blockingQuery.GetBlockingUser()),
			zap.String("blocked_sql_id", blockingQuery.GetBlockedSQLID()),
			zap.Float64("blocked_wait_seconds", blockingQuery.BlockedWaitSec.Float64))

		// Extract attribute values with null handling
		blockedUser := blockingQuery.GetBlockedUser()
		blockingUser := blockingQuery.GetBlockingUser()
		blockedSQLID := blockingQuery.GetBlockedSQLID()
		databaseName := blockingQuery.GetDatabaseName()
		blockedQueryText := commonutils.AnonymizeAndNormalize(blockingQuery.GetBlockedQueryText())

		// Convert numeric values to strings for attributes
		blockedSID := ""
		if blockingQuery.BlockedSID.Valid {
			blockedSID = fmt.Sprintf("%d", blockingQuery.BlockedSID.Int64)
		}

		blockingSID := ""
		if blockingQuery.BlockingSID.Valid {
			blockingSID = fmt.Sprintf("%d", blockingQuery.BlockingSID.Int64)
		}

		blockedSerial := ""
		if blockingQuery.BlockedSerial.Valid {
			blockedSerial = fmt.Sprintf("%d", blockingQuery.BlockedSerial.Int64)
		}

		blockingSerial := ""
		if blockingQuery.BlockingSerial.Valid {
			blockingSerial = fmt.Sprintf("%d", blockingQuery.BlockingSerial.Int64)
		}

		s.logger.Debug("Collected blocking query metrics",
			zap.String("blocked_user", blockedUser),
			zap.String("blocking_user", blockingUser),
			zap.String("blocked_sql_id", blockedSQLID),
			zap.String("database_name", databaseName),
			zap.String("blocked_sid", blockedSID),
			zap.String("blocking_sid", blockingSID),
			zap.String("blocked_serial", blockedSerial),
			zap.String("blocking_serial", blockingSerial),
			zap.Float64("blocked_wait_seconds", blockingQuery.BlockedWaitSec.Float64))

		// Record only wait time metric with all other values as attributes
		if blockingQuery.BlockedWaitSec.Valid {
			s.mb.RecordNewrelicoracledbBlockingQueriesWaitTimeDataPoint(
				now,
				blockingQuery.BlockedWaitSec.Float64,
				s.instanceName, // instanceIDAttributeValue
				blockedUser,
				blockingUser,
				blockedSQLID,
				blockedSID,
				blockingSID,
				blockedSerial,
				blockingSerial,
				blockedQueryText,
				databaseName,
			)
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating through blocking queries rows", zap.Error(err))
		scrapeErrors = append(scrapeErrors, err)
	}

	s.logger.Debug("Completed blocking queries scrape")
	return scrapeErrors
}
