// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
)

// SessionScraper handles session count Oracle metric
type SessionScraper struct {
	client       models.DbClient
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewSessionScraper creates a new session scraper
func NewSessionScraper(client models.DbClient, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *SessionScraper {
	return &SessionScraper{
		client:       client,
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
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing session count query: %w", err))
		return errors
	}

	for _, row := range rows {
		sessionCountStr := row["SESSION_COUNT"]
		sessionCount, parseErr := strconv.ParseInt(sessionCountStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse session count value: %s, error: %w", sessionCountStr, parseErr))
			continue
		}
		
		s.mb.RecordNewrelicoracledbSessionsCountDataPoint(now, sessionCount, s.instanceName)
		s.logger.Debug("Collected Oracle session count", zap.Int64("count", sessionCount))
	}
	
	return errors
}
