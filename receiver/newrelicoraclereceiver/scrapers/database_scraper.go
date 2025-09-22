// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

// DatabaseScraper handles all database-related Oracle metrics
type DatabaseScraper struct {
	client       models.DbClient
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewDatabaseScraper creates a new database scraper
func NewDatabaseScraper(client models.DbClient, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *DatabaseScraper {
	return &DatabaseScraper{
		client:       client,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeDbSize collects Oracle database size metrics
func (s *DatabaseScraper) ScrapeDbSize(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database size")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database size query: %w", err))
		return errors
	}

	for _, row := range rows {
		sizeStr := row["SIZE_BYTES"]
		sizeBytes, parseErr := strconv.ParseInt(sizeStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse database size: %s, error: %w", sizeStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle database size", zap.Int64("size_bytes", sizeBytes))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbDatabaseSizeDataPoint(now, sizeBytes, s.instanceName)
	}
	
	return errors
}

// ScrapeLogicalReads collects Oracle logical reads metrics
func (s *DatabaseScraper) ScrapeLogicalReads(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle logical reads")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing logical reads query: %w", err))
		return errors
	}

	for _, row := range rows {
		logicalReadsStr := row["LOGICAL_READS"]
		logicalReads, parseErr := strconv.ParseInt(logicalReadsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse logical reads: %s, error: %w", logicalReadsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle logical reads", zap.Int64("logical_reads", logicalReads))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbLogicalReadsDataPoint(now, logicalReads, s.instanceName)
	}
	
	return errors
}

// ScrapePhysicalReads collects Oracle physical reads metrics
func (s *DatabaseScraper) ScrapePhysicalReads(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle physical reads")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing physical reads query: %w", err))
		return errors
	}

	for _, row := range rows {
		physicalReadsStr := row["PHYSICAL_READS"]
		physicalReads, parseErr := strconv.ParseInt(physicalReadsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse physical reads: %s, error: %w", physicalReadsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle physical reads", zap.Int64("physical_reads", physicalReads))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbPhysicalReadsDataPoint(now, physicalReads, s.instanceName)
	}
	
	return errors
}

// ScrapeDbConnections collects Oracle database connections metrics
func (s *DatabaseScraper) ScrapeDbConnections(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database connections")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database connections query: %w", err))
		return errors
	}

	for _, row := range rows {
		connectionsStr := row["CONNECTIONS"]
		connections, parseErr := strconv.ParseInt(connectionsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse connections count: %s, error: %w", connectionsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle database connections", zap.Int64("connections", connections))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbConnectionsDataPoint(now, connections, s.instanceName)
	}
	
	return errors
}

// ScrapeDbStatus collects Oracle database status metrics
func (s *DatabaseScraper) ScrapeDbStatus(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database status")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database status query: %w", err))
		return errors
	}

	for _, row := range rows {
		status := row["STATUS"]
		databaseRole := row["DATABASE_ROLE"]
		
		s.logger.Debug("Collected Oracle database status", 
			zap.String("status", status),
			zap.String("database_role", databaseRole))
		
		// For status metrics, you might want to convert string values to numeric
		var statusValue int64
		switch status {
		case "OPEN":
			statusValue = 1
		case "MOUNTED":
			statusValue = 2
		case "CLOSED":
			statusValue = 0
		default:
			statusValue = -1
		}
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbDatabaseStatusDataPoint(now, statusValue, status, databaseRole, s.instanceName)
	}
	
	return errors
}

// ScrapeTransactionRate collects Oracle transaction rate metrics
func (s *DatabaseScraper) ScrapeTransactionRate(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle transaction rate")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing transaction rate query: %w", err))
		return errors
	}

	for _, row := range rows {
		transactionRateStr := row["TRANSACTION_RATE"]
		transactionRate, parseErr := strconv.ParseFloat(transactionRateStr, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse transaction rate: %s, error: %w", transactionRateStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle transaction rate", zap.Float64("transaction_rate", transactionRate))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbTransactionRateDataPoint(now, transactionRate, s.instanceName)
	}
	
	return errors
}

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

// DatabaseScraper handles all database-related Oracle metrics
type DatabaseScraper struct {
	client       models.DbClient
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewDatabaseScraper creates a new database scraper
func NewDatabaseScraper(client models.DbClient, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *DatabaseScraper {
	return &DatabaseScraper{
		client:       client,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeDbSize collects Oracle database size metrics
func (s *DatabaseScraper) ScrapeDbSize(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database size")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database size query: %w", err))
		return errors
	}

	for _, row := range rows {
		sizeStr := row["SIZE_BYTES"]
		sizeBytes, parseErr := strconv.ParseInt(sizeStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse database size: %s, error: %w", sizeStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle database size", zap.Int64("size_bytes", sizeBytes))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbDatabaseSizeDataPoint(now, sizeBytes, s.instanceName)
	}
	
	return errors
}

// ScrapeLogicalReads collects Oracle logical reads metrics
func (s *DatabaseScraper) ScrapeLogicalReads(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle logical reads")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing logical reads query: %w", err))
		return errors
	}

	for _, row := range rows {
		logicalReadsStr := row["LOGICAL_READS"]
		logicalReads, parseErr := strconv.ParseInt(logicalReadsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse logical reads: %s, error: %w", logicalReadsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle logical reads", zap.Int64("logical_reads", logicalReads))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbLogicalReadsDataPoint(now, logicalReads, s.instanceName)
	}
	
	return errors
}

// ScrapePhysicalReads collects Oracle physical reads metrics
func (s *DatabaseScraper) ScrapePhysicalReads(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle physical reads")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing physical reads query: %w", err))
		return errors
	}

	for _, row := range rows {
		physicalReadsStr := row["PHYSICAL_READS"]
		physicalReads, parseErr := strconv.ParseInt(physicalReadsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse physical reads: %s, error: %w", physicalReadsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle physical reads", zap.Int64("physical_reads", physicalReads))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbPhysicalReadsDataPoint(now, physicalReads, s.instanceName)
	}
	
	return errors
}

// ScrapeDbConnections collects Oracle database connections metrics
func (s *DatabaseScraper) ScrapeDbConnections(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database connections")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database connections query: %w", err))
		return errors
	}

	for _, row := range rows {
		connectionsStr := row["CONNECTIONS"]
		connections, parseErr := strconv.ParseInt(connectionsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse connections count: %s, error: %w", connectionsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle database connections", zap.Int64("connections", connections))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbConnectionsDataPoint(now, connections, s.instanceName)
	}
	
	return errors
}

// ScrapeDbStatus collects Oracle database status metrics
func (s *DatabaseScraper) ScrapeDbStatus(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database status")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database status query: %w", err))
		return errors
	}

	for _, row := range rows {
		status := row["STATUS"]
		databaseRole := row["DATABASE_ROLE"]
		
		s.logger.Debug("Collected Oracle database status", 
			zap.String("status", status),
			zap.String("database_role", databaseRole))
		
		// For status metrics, you might want to convert string values to numeric
		var statusValue int64
		switch status {
		case "OPEN":
			statusValue = 1
		case "MOUNTED":
			statusValue = 2
		case "CLOSED":
			statusValue = 0
		default:
			statusValue = -1
		}
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbDatabaseStatusDataPoint(now, statusValue, status, databaseRole, s.instanceName)
	}
	
	return errors
}

// ScrapeTransactionRate collects Oracle transaction rate metrics
func (s *DatabaseScraper) ScrapeTransactionRate(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle transaction rate")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.client.MetricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing transaction rate query: %w", err))
		return errors
	}

	for _, row := range rows {
		transactionRateStr := row["TRANSACTION_RATE"]
		transactionRate, parseErr := strconv.ParseFloat(transactionRateStr, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse transaction rate: %s, error: %w", transactionRateStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle transaction rate", zap.Float64("transaction_rate", transactionRate))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbTransactionRateDataPoint(now, transactionRate, s.instanceName)
	}
	
	return errors
}
