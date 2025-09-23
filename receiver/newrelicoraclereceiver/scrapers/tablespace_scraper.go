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

// TablespaceScraper handles Oracle tablespace metrics
type TablespaceScraper struct {
	db           *sql.DB
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewTablespaceScraper creates a new tablespace scraper
func NewTablespaceScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *TablespaceScraper {
	return &TablespaceScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeTablespaceMetrics collects Oracle tablespace metrics
func (s *TablespaceScraper) ScrapeTablespaceMetrics(ctx context.Context) []error {
	var errors []error

	s.logger.Debug("Scraping Oracle tablespace metrics")
	now := pcommon.NewTimestampFromTime(time.Now())

	// Scrape main tablespace metrics
	errors = append(errors, s.scrapeTablespaceUsageMetrics(ctx, now)...)

	// Scrape global name tablespace metrics
	errors = append(errors, s.scrapeGlobalNameTablespaceMetrics(ctx, now)...)

	return errors
}

// scrapeTablespaceUsageMetrics handles the main tablespace usage metrics
func (s *TablespaceScraper) scrapeTablespaceUsageMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute tablespace metrics query directly using the shared DB connection
	s.logger.Debug("Executing tablespace metrics query", zap.String("sql", queries.TablespaceMetricsSQL))

	rows, err := s.db.QueryContext(ctx, queries.TablespaceMetricsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing tablespace metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var tablespaceName string
		var usedPercent, used, size, offline float64

		err := rows.Scan(&tablespaceName, &usedPercent, &used, &size, &offline)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning tablespace row: %w", err))
			continue
		}

		// Record tablespace metrics using the proper generated methods
		s.logger.Info("Tablespace metrics collected",
			zap.String("tablespace", tablespaceName),
			zap.Float64("used_percent", usedPercent),
			zap.Float64("used_bytes", used),
			zap.Float64("size_bytes", size),
			zap.Float64("offline", offline),
			zap.String("instance", s.instanceName),
		)

		// Record all tablespace metrics using the proper metadata builder methods
		s.mb.RecordNewrelicoracledbTablespaceSpaceConsumedBytesDataPoint(now, int64(used), s.instanceName, tablespaceName)
		s.mb.RecordNewrelicoracledbTablespaceSpaceReservedBytesDataPoint(now, int64(size), s.instanceName, tablespaceName)
		s.mb.RecordNewrelicoracledbTablespaceSpaceUsedPercentageDataPoint(now, int64(usedPercent), s.instanceName, tablespaceName)
		s.mb.RecordNewrelicoracledbTablespaceIsOfflineDataPoint(now, int64(offline), s.instanceName, tablespaceName)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating tablespace rows: %w", err))
	}

	s.logger.Debug("Collected Oracle tablespace metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeGlobalNameTablespaceMetrics handles the global name tablespace metrics
func (s *TablespaceScraper) scrapeGlobalNameTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute global name tablespace query
	s.logger.Debug("Executing global name tablespace query", zap.String("sql", queries.GlobalNameTablespaceSQL))

	rows, err := s.db.QueryContext(ctx, queries.GlobalNameTablespaceSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing global name tablespace query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var tablespaceName, globalName string

		err := rows.Scan(&tablespaceName, &globalName)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning global name tablespace row: %w", err))
			continue
		}

		// Record global name metrics
		s.logger.Info("Global name tablespace metrics collected",
			zap.String("tablespace", tablespaceName),
			zap.String("global_name", globalName),
			zap.String("instance", s.instanceName),
		)

		// Record the global name metric (using 1 as value since it's an attribute metric)
		s.mb.RecordNewrelicoracledbTablespaceGlobalNameDataPoint(now, 1, s.instanceName, tablespaceName)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating global name tablespace rows: %w", err))
	}

	s.logger.Debug("Collected Oracle global name tablespace metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}
