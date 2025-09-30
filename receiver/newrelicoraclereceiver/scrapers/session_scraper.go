// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
) // SessionScraper handles session count Oracle metric
type SessionScraper struct {
	db           *sql.DB // Direct DB connection passed from main scraper
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewSessionScraper creates a new session scraper
func NewSessionScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *SessionScraper {
	return &SessionScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeSessionCount collects Oracle session count metric
func (s *SessionScraper) ScrapeSessionCount(ctx context.Context) []error {
	var errs []error

	if !s.config.Metrics.NewrelicoracledbSessionsCount.Enabled {
		return errs
	}

	s.logger.Debug("Scraping Oracle session count")
	now := pcommon.NewTimestampFromTime(time.Now())

	// Execute session count query directly using the shared DB connection
	s.logger.Debug("Executing session count query",
		zap.String("sql", errors.FormatQueryForLogging(queries.SessionCountSQL)))

	var sessionCount int64
	err := s.db.QueryRowContext(ctx, queries.SessionCountSQL).Scan(&sessionCount)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Warn("No rows returned from session count query")
			return errs
		}

		// Create structured error with context
		scraperErr := errors.NewQueryError(
			"session_count_query",
			queries.SessionCountSQL,
			err,
			map[string]interface{}{
				"instance":  s.instanceName,
				"retryable": errors.IsRetryableError(err),
				"permanent": errors.IsPermanentError(err),
			},
		)

		// Log error with appropriate level based on error type
		if errors.IsPermanentError(err) {
			s.logger.Error("Permanent error executing session count query",
				zap.Error(scraperErr),
				zap.String("instance", s.instanceName))
		} else if errors.IsRetryableError(err) {
			s.logger.Warn("Retryable error executing session count query",
				zap.Error(scraperErr),
				zap.String("instance", s.instanceName))
		} else {
			s.logger.Error("Error executing session count query",
				zap.Error(scraperErr),
				zap.String("instance", s.instanceName))
		}

		errs = append(errs, scraperErr)
		return errs
	}

	// Record the metric
	s.mb.RecordNewrelicoracledbSessionsCountDataPoint(now, sessionCount, s.instanceName)
	s.logger.Debug("Collected Oracle session count",
		zap.Int64("count", sessionCount),
		zap.String("instance", s.instanceName))

	return errs
}
