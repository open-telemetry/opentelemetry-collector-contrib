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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// ExecutionPlanScraper handles scraping execution plans for SQL IDs obtained from slow queries
type ExecutionPlanScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewExecutionPlanScraper creates a new ExecutionPlanScraper instance
func NewExecutionPlanScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *ExecutionPlanScraper {
	return &ExecutionPlanScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// ScrapeExecutionPlans fetches execution plans for the provided SQL IDs with performance optimizations
func (s *ExecutionPlanScraper) ScrapeExecutionPlans(ctx context.Context, sqlIDs []string) []error {
	var errs []error

	// Skip if no SQL IDs provided
	if len(sqlIDs) == 0 {
		s.logger.Debug("No SQL IDs provided for execution plan scraping")
		return errs
	}

	s.logger.Debug("Starting execution plan scraping", 
		zap.Int("sql_ids_count", len(sqlIDs)),
		zap.Strings("sql_ids", sqlIDs))

	// Performance optimization: Process in batches to avoid overwhelming the database
	const batchSize = 3  // Conservative batch size for performance
	totalProcessed := 0
	
	for i := 0; i < len(sqlIDs); i += batchSize {
		end := min(i+batchSize, len(sqlIDs))
		batch := sqlIDs[i:end]
		
		s.logger.Debug("Processing execution plan batch",
			zap.Int("batch_number", i/batchSize+1),
			zap.Int("batch_size", len(batch)),
			zap.Strings("batch_sql_ids", batch))

		batchErrs := s.processBatch(ctx, batch)
		errs = append(errs, batchErrs...)
		totalProcessed += len(batch)
		
		// Check for context cancellation between batches
		select {
		case <-ctx.Done():
			s.logger.Debug("Context cancelled during batch processing")
			return append(errs, fmt.Errorf("execution plan batch processing cancelled: %w", ctx.Err()))
		default:
		}
	}

	s.logger.Info("Execution plan scraping completed",
		zap.Int("total_sql_ids_processed", totalProcessed),
		zap.Int("total_errors", len(errs)))

	return errs
}

// processBatch processes a single batch of SQL IDs with timeout and performance monitoring
func (s *ExecutionPlanScraper) processBatch(ctx context.Context, sqlIDs []string) []error {
	var errs []error
	
	// Add timeout to prevent hanging queries
	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	startTime := time.Now()

	// Get the execution plan query with SQL IDs
	query, args := queries.GetExecutionPlanQuery(sqlIDs)
	if query == "" {
		s.logger.Warn("Empty execution plan query generated for batch")
		return errs
	}

	s.logger.Debug("Executing execution plan query", 
		zap.String("query_preview", query[:min(200, len(query))]),
		zap.Int("args_count", len(args)))

	// Execute the query with timeout
	rows, err := s.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		errMsg := fmt.Errorf("failed to execute execution plan query: %w", err)
		s.logger.Error("Execution plan query failed", zap.Error(errMsg))
		return []error{errMsg}
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close execution plan query rows", zap.Error(closeErr))
		}
	}()

	// Process results
	planCount := 0
	for rows.Next() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			s.logger.Debug("Context cancelled during execution plan processing")
			return append(errs, fmt.Errorf("execution plan processing cancelled: %w", ctx.Err()))
		default:
		}

		var plan models.ExecutionPlan
		
		// Scan the row into execution plan model
		err := rows.Scan(
			&plan.DatabaseName,
			&plan.QueryID,
			&plan.PlanHashValue,
			&plan.ExecutionPlanXML,
		)
		if err != nil {
			errMsg := fmt.Errorf("failed to scan execution plan row: %w", err)
			s.logger.Error("Failed to scan execution plan", zap.Error(errMsg))
			errs = append(errs, errMsg)
			continue
		}

		// Validate the execution plan
		if !plan.IsValidForMetrics() {
			s.logger.Debug("Skipping invalid execution plan",
				zap.String("query_id", plan.GetQueryID()),
				zap.Int64("plan_hash_value", plan.GetPlanHashValue()))
			continue
		}

		planCount++
		s.logger.Debug("Processed execution plan",
			zap.Int("plan_number", planCount),
			zap.String("database_name", plan.GetDatabaseName()),
			zap.String("query_id", plan.GetQueryID()),
			zap.Int64("plan_hash_value", plan.GetPlanHashValue()),
			zap.Int("xml_length", len(plan.GetExecutionPlanXML())))

		// Build and emit metrics for the execution plan
		s.buildExecutionPlanMetrics(&plan)
	}

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		errMsg := fmt.Errorf("error during execution plan rows iteration: %w", err)
		s.logger.Error("Execution plan rows iteration failed", zap.Error(errMsg))
		errs = append(errs, errMsg)
	}

	s.logger.Info("Execution plan scraping completed",
		zap.Int("total_plans_processed", planCount),
		zap.Int("input_sql_ids", len(sqlIDs)),
		zap.Int("errors_count", len(errs)))

	return errs
}

// buildExecutionPlanMetrics creates metrics from execution plan data
func (s *ExecutionPlanScraper) buildExecutionPlanMetrics(plan *models.ExecutionPlan) {
	// Only build metrics if execution plan metrics are enabled
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbExecutionPlanInfo.Enabled {
		return
	}

	s.logger.Debug("Building execution plan metrics",
		zap.String("query_id", plan.GetQueryID()),
		zap.Int64("plan_hash_value", plan.GetPlanHashValue()))

	// Record execution plan info metric with comprehensive XML data
	s.mb.RecordNewrelicoracledbExecutionPlanInfoDataPoint(
		pcommon.NewTimestampFromTime(time.Now()), // Current timestamp
		1, // Set to 1 to indicate presence of execution plan
		plan.GetDatabaseName(),
		plan.GetQueryID(),
		fmt.Sprintf("%d", plan.GetPlanHashValue()),
		plan.GetExecutionPlanXML(), // This contains the comprehensive XML execution plan
	)
}

// min returns the minimum of two integers (helper function for Go versions < 1.21)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
