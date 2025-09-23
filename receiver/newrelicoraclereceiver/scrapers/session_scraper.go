// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// SessionScraper handles session count Oracle metric
type SessionScraper struct {
	db           *sql.DB          // Direct DB connection passed from main scraper
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
	var errors []error
	
	if !s.config.Metrics.NewrelicoracledbSessionsCount.Enabled {
		return errors
	}

	s.logger.Debug("Scraping Oracle session count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	// Execute session count query directly using the shared DB connection
	s.logger.Debug("Executing session count query", zap.String("sql", queries.SessionCountSQL))
	
	var sessionCount int64
	err := s.db.QueryRowContext(ctx, queries.SessionCountSQL).Scan(&sessionCount)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Warn("No rows returned from session count query")
			return errors
		}
		errors = append(errors, fmt.Errorf("error executing session count query: %w", err))
		return errors
	}
	
	// Record the metric
	s.mb.RecordNewrelicoracledbSessionsCountDataPoint(now, sessionCount, s.instanceName)
	s.logger.Debug("Collected Oracle session count", zap.Int64("count", sessionCount), zap.String("instance", s.instanceName))
	
	return errors
}
