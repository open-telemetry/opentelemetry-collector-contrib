// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// UserConnectionScraper handles scraping of SQL Server user connection metrics
type UserConnectionScraper struct {
	connection    SQLConnectionInterface
	logger        *zap.Logger
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewUserConnectionScraper creates a new UserConnectionScraper instance
func NewUserConnectionScraper(connection SQLConnectionInterface, logger *zap.Logger, engineEdition int) *UserConnectionScraper {
	return &UserConnectionScraper{
		connection:    connection,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeUserConnectionStatusMetrics collects user connection status distribution metrics
// This method retrieves the count of user connections grouped by their current status
func (s *UserConnectionScraper) ScrapeUserConnectionStatusMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection status metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.status.metrics")
	if !found {
		return fmt.Errorf("no user connection status metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing user connection status metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.UserConnectionStatusMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute user connection status query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute user connection status query: %w", err)
	}

	// If no results, this may indicate no user connections or query issues
	if len(results) == 0 {
		s.logger.Debug("No user connection status metrics found - may indicate no active user connections")
		return nil
	}

	s.logger.Debug("Processing user connection status metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each status result
	for _, result := range results {
		if err := s.processUserConnectionStatusMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process user connection status metrics",
				zap.Error(err),
				zap.String("status", result.Status))
			return err
		}
	}

	return nil
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition with Default fallback
func (s *UserConnectionScraper) getQueryForMetric(metricName string) (string, bool) {
	query, found := queries.GetQueryForMetric(queries.UserConnectionQueries, metricName, s.engineEdition)
	if found {
		s.logger.Debug("Using query for metric",
			zap.String("metric_name", metricName),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
	}
	return query, found
}

// processUserConnectionStatusMetrics processes user connection status metrics and creates OpenTelemetry metrics
func (s *UserConnectionScraper) processUserConnectionStatusMetrics(result models.UserConnectionStatusMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Process SessionCount as a gauge metric
	if result.SessionCount != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.status.count")
		metric.SetDescription("Number of user connections by status")
		metric.SetUnit("1")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.SessionCount)

		// Add attributes
		attrs := dataPoint.Attributes()
		attrs.PutStr("status", result.Status)
		attrs.PutStr("source_type", "user_connections")
	}

	return nil
}

// ScrapeLoginLogoutMetrics collects login and logout rate metrics
// This method retrieves authentication activity counters from performance counters
func (s *UserConnectionScraper) ScrapeLoginLogoutMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server login/logout rate metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.authentication.metrics")
	if !found {
		return fmt.Errorf("no login/logout rate metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing login/logout rate metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.LoginLogoutMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute login/logout rate query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute login/logout rate query: %w", err)
	}

	// If no results, this may indicate no authentication activity or query issues
	if len(results) == 0 {
		s.logger.Debug("No login/logout rate metrics found - may indicate no recent authentication activity")
		return nil
	}

	s.logger.Debug("Processing login/logout rate metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each authentication result
	for _, result := range results {
		if err := s.processLoginLogoutMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process login/logout rate metrics",
				zap.Error(err),
				zap.String("counter_name", result.CounterName))
			return err
		}
	}

	return nil
}

// ScrapeLoginLogoutSummaryMetrics collects aggregated authentication activity statistics
func (s *UserConnectionScraper) ScrapeLoginLogoutSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server login/logout summary metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.authentication.summary")
	if !found {
		return fmt.Errorf("no login/logout summary metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing login/logout summary metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.LoginLogoutSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute login/logout summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute login/logout summary query: %w", err)
	}

	// If no results, this may indicate no authentication activity or query issues
	if len(results) == 0 {
		s.logger.Debug("No login/logout summary metrics found - may indicate no recent authentication activity")
		return nil
	}

	s.logger.Debug("Processing login/logout summary metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each summary result
	for _, result := range results {
		if err := s.processLoginLogoutSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process login/logout summary metrics",
				zap.Error(err))
			return err
		}
	}

	return nil
}

// processLoginLogoutMetrics processes login/logout rate metrics and creates OpenTelemetry metrics
func (s *UserConnectionScraper) processLoginLogoutMetrics(result models.LoginLogoutMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Process CntrValue as a gauge metric
	if result.CntrValue != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.rate")
		metric.SetDescription("SQL Server authentication rate per second (logins and logouts)")
		metric.SetUnit("1/s")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.CntrValue)

		// Add attributes to identify the counter type and user information
		attrs := dataPoint.Attributes()
		attrs.PutStr("counter_name", result.CounterName)
		attrs.PutStr("source_type", "user_connections")

		// Add username if available
		if result.Username != nil {
			attrs.PutStr("username", *result.Username)
		}

		// Add source IP if available
		if result.SourceIP != nil {
			attrs.PutStr("source_ip", *result.SourceIP)
		}
	}

	return nil
}

// processLoginLogoutSummaryMetrics processes login/logout summary metrics and creates OpenTelemetry metrics
func (s *UserConnectionScraper) processLoginLogoutSummaryMetrics(result models.LoginLogoutSummary, scopeMetrics pmetric.ScopeMetrics) error {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	// Helper function to add common attributes
	addCommonAttributes := func(attrs pcommon.Map) {
		attrs.PutStr("source_type", "user_connections")
		if result.Username != nil {
			attrs.PutStr("username", *result.Username)
		}
		if result.SourceIP != nil {
			attrs.PutStr("source_ip", *result.SourceIP)
		}
	}

	// Process LoginsPerSec
	if result.LoginsPerSec != nil && *result.LoginsPerSec > 0 {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.logins_per_sec")
		metric.SetDescription("SQL Server login rate per second")
		metric.SetUnit("1/s")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(timestamp)
		dataPoint.SetIntValue(*result.LoginsPerSec)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process LogoutsPerSec
	if result.LogoutsPerSec != nil && *result.LogoutsPerSec > 0 {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.logouts_per_sec")
		metric.SetDescription("SQL Server logout rate per second")
		metric.SetUnit("1/s")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(timestamp)
		dataPoint.SetIntValue(*result.LogoutsPerSec)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process TotalAuthActivity
	if result.TotalAuthActivity != nil && *result.TotalAuthActivity > 0 {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.total_activity")
		metric.SetDescription("SQL Server total authentication activity per second (logins + logouts)")
		metric.SetUnit("1/s")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(timestamp)
		dataPoint.SetIntValue(*result.TotalAuthActivity)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process ConnectionChurnRate
	if result.ConnectionChurnRate != nil && *result.ConnectionChurnRate >= 0 {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.churn_rate")
		metric.SetDescription("SQL Server connection churn rate (logout/login ratio)")
		metric.SetUnit("%")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(timestamp)
		dataPoint.SetDoubleValue(*result.ConnectionChurnRate)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	return nil
}

// ScrapeFailedLoginMetrics collects failed login attempts from SQL Server error log
// This method retrieves failed login messages from the error log for security monitoring
func (s *UserConnectionScraper) ScrapeFailedLoginMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server failed login metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.failed_logins.metrics")
	if !found {
		return fmt.Errorf("no failed login metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing failed login metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailedLoginMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute failed login query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute failed login query: %w", err)
	}

	s.logger.Debug("Processing failed login metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each failed login result
	for _, result := range results {
		if err := s.processFailedLoginMetrics(result, scopeMetrics); err != nil {
			logDate := ""
			if result.LogDate != nil {
				logDate = *result.LogDate
			}
			s.logger.Error("Failed to process failed login metrics",
				zap.Error(err),
				zap.String("log_date", logDate))
			return err
		}
	}

	return nil
}

// ScrapeFailedLoginSummaryMetrics collects aggregated failed login statistics
func (s *UserConnectionScraper) ScrapeFailedLoginSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server failed login summary metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.failed_logins_summary.metrics")
	if !found {
		return fmt.Errorf("no failed login summary metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing failed login summary metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailedLoginSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute failed login summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute failed login summary query: %w", err)
	}

	s.logger.Debug("Processing failed login summary metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each summary result
	for _, result := range results {
		if err := s.processFailedLoginSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process failed login summary metrics",
				zap.Error(err))
			return err
		}
	}

	return nil
}

// processFailedLoginMetrics processes failed login metrics and creates OpenTelemetry metrics
func (s *UserConnectionScraper) processFailedLoginMetrics(result models.FailedLoginMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Create a counter metric for failed login events
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("sqlserver.user_connections.authentication.failed_login_event")
	metric.SetDescription("SQL Server failed login attempt event")
	metric.SetUnit("1")

	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dataPoint.SetIntValue(1) // Each record represents one failed login

	// Add attributes with failed login details
	attrs := dataPoint.Attributes()
	attrs.PutStr("source_type", "user_connections")

	// Add extracted username and source IP if available
	if result.Username != nil {
		attrs.PutStr("username", *result.Username)
	}
	if result.SourceIP != nil {
		attrs.PutStr("source_ip", *result.SourceIP)
	}

	// Handle different query formats based on engine edition
	if s.engineEdition == queries.AzureSQLDatabaseEngineEdition {
		// Azure SQL Database format
		if result.EventType != nil {
			attrs.PutStr("event_type", *result.EventType)
		}
		if result.Description != nil {
			attrs.PutStr("description", *result.Description)
		}
		if result.StartTime != nil {
			attrs.PutStr("start_time", *result.StartTime)
		}
		if result.ClientIP != nil {
			attrs.PutStr("client_ip", *result.ClientIP)
		}
	} else {
		// Standard SQL Server format using sp_readerrorlog
		if result.LogDate != nil {
			attrs.PutStr("log_date", *result.LogDate)
		}
		if result.ProcessInfo != nil {
			attrs.PutStr("process_info", *result.ProcessInfo)
		}
		if result.Text != nil {
			attrs.PutStr("error_text", *result.Text)
		}
	}

	return nil
}

// ScrapeUserConnectionStatsMetrics scrapes user connection statistical analysis metrics
func (s *UserConnectionScraper) ScrapeUserConnectionStatsMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection stats metrics")

	// Scrape summary metrics
	if err := s.ScrapeUserConnectionSummaryMetrics(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape user connection summary metrics", zap.Error(err))
		return fmt.Errorf("failed to scrape user connection summary metrics: %w", err)
	}

	// Scrape utilization metrics
	if err := s.ScrapeUserConnectionUtilizationMetrics(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape user connection utilization metrics", zap.Error(err))
		return fmt.Errorf("failed to scrape user connection utilization metrics: %w", err)
	}

	// Scrape client breakdown metrics
	if err := s.ScrapeUserConnectionByClientMetrics(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape user connection by client metrics", zap.Error(err))
		return fmt.Errorf("failed to scrape user connection by client metrics: %w", err)
	}

	// Scrape client summary metrics
	if err := s.ScrapeUserConnectionClientSummaryMetrics(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape user connection client summary metrics", zap.Error(err))
		return fmt.Errorf("failed to scrape user connection client summary metrics: %w", err)
	}

	return nil
}

// ScrapeUserConnectionSummaryMetrics scrapes user connection summary metrics
func (s *UserConnectionScraper) ScrapeUserConnectionSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection summary metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.status.summary")
	if !found {
		return fmt.Errorf("no user connection summary metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing user connection summary metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.UserConnectionStatusSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute user connection summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute user connection summary query: %w", err)
	}

	// If no results, this may indicate no user connections
	if len(results) == 0 {
		s.logger.Debug("No user connection summary metrics found")
		return nil
	}

	s.logger.Debug("Processing user connection summary metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each summary result
	for _, result := range results {
		if err := s.processUserConnectionSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process user connection summary metrics",
				zap.Error(err))
			return err
		}
	}

	return nil
}

// ScrapeUserConnectionUtilizationMetrics scrapes user connection utilization metrics
func (s *UserConnectionScraper) ScrapeUserConnectionUtilizationMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection utilization metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.utilization")
	if !found {
		return fmt.Errorf("no user connection utilization metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing user connection utilization metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.UserConnectionUtilization
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute user connection utilization query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute user connection utilization query: %w", err)
	}

	// If no results, this may indicate no user connections
	if len(results) == 0 {
		s.logger.Debug("No user connection utilization metrics found")
		return nil
	}

	s.logger.Debug("Processing user connection utilization metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each utilization result
	for _, result := range results {
		if err := s.processUserConnectionUtilizationMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process user connection utilization metrics",
				zap.Error(err))
			return err
		}
	}

	return nil
}

// ScrapeUserConnectionByClientMetrics scrapes user connection by client metrics
func (s *UserConnectionScraper) ScrapeUserConnectionByClientMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection by client metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.by_client")
	if !found {
		return fmt.Errorf("no user connection by client metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing user connection by client metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.UserConnectionByClientMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute user connection by client query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute user connection by client query: %w", err)
	}

	// If no results, this may indicate no user connections
	if len(results) == 0 {
		s.logger.Debug("No user connection by client metrics found")
		return nil
	}

	s.logger.Debug("Processing user connection by client metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each client result
	for _, result := range results {
		if err := s.processUserConnectionByClientMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process user connection by client metrics",
				zap.Error(err),
				zap.String("program_name", result.ProgramName))
			return err
		}
	}

	return nil
}

// ScrapeUserConnectionClientSummaryMetrics scrapes user connection client summary metrics
func (s *UserConnectionScraper) ScrapeUserConnectionClientSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server user connection client summary metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.user_connections.client.summary")
	if !found {
		return fmt.Errorf("no user connection client summary metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing user connection client summary metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.UserConnectionClientSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute user connection client summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute user connection client summary query: %w", err)
	}

	// If no results, this may indicate no user connections
	if len(results) == 0 {
		s.logger.Debug("No user connection client summary metrics found")
		return nil
	}

	s.logger.Debug("Processing user connection client summary metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each client summary result
	for _, result := range results {
		if err := s.processUserConnectionClientSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process user connection client summary metrics",
				zap.Error(err))
			return err
		}
	}

	return nil
}

// processUserConnectionSummaryMetrics processes user connection summary metrics
func (s *UserConnectionScraper) processUserConnectionSummaryMetrics(result models.UserConnectionStatusSummary, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Processing user connection summary metrics")

	// Process total user connections
	if result.TotalUserConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.total")
		metric.SetDescription("Total number of user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.TotalUserConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process sleeping connections
	if result.SleepingConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.sleeping")
		metric.SetDescription("Number of sleeping user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.SleepingConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process running connections
	if result.RunningConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.running")
		metric.SetDescription("Number of running user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.RunningConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process suspended connections
	if result.SuspendedConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.suspended")
		metric.SetDescription("Number of suspended user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.SuspendedConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process runnable connections
	if result.RunnableConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.runnable")
		metric.SetDescription("Number of runnable user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.RunnableConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process dormant connections
	if result.DormantConnections != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.dormant")
		metric.SetDescription("Number of dormant user connections")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.DormantConnections)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	return nil
}

// processUserConnectionUtilizationMetrics processes user connection utilization metrics
func (s *UserConnectionScraper) processUserConnectionUtilizationMetrics(result models.UserConnectionUtilization, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Processing user connection utilization metrics")

	// Process active connection ratio
	if result.ActiveConnectionRatio != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.utilization.active_ratio")
		metric.SetDescription("Active connection ratio")
		metric.SetUnit("ratio")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetDoubleValue(*result.ActiveConnectionRatio)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process idle connection ratio
	if result.IdleConnectionRatio != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.utilization.idle_ratio")
		metric.SetDescription("Idle connection ratio")
		metric.SetUnit("ratio")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetDoubleValue(*result.IdleConnectionRatio)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process waiting connection ratio
	if result.WaitingConnectionRatio != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.utilization.waiting_ratio")
		metric.SetDescription("Waiting connection ratio")
		metric.SetUnit("ratio")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetDoubleValue(*result.WaitingConnectionRatio)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process connection efficiency
	if result.ConnectionEfficiency != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.utilization.efficiency")
		metric.SetDescription("Connection efficiency score")
		metric.SetUnit("ratio")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetDoubleValue(*result.ConnectionEfficiency)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	return nil
}

// processUserConnectionByClientMetrics processes user connection by client metrics
func (s *UserConnectionScraper) processUserConnectionByClientMetrics(result models.UserConnectionByClientMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Processing user connection by client metrics",
		zap.String("host_name", result.HostName),
		zap.String("program_name", result.ProgramName))

	if result.ConnectionCount != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.count")
		metric.SetDescription("Number of connections by client host and program")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.ConnectionCount)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
		attrs.PutStr("host_name", result.HostName)
		attrs.PutStr("program_name", result.ProgramName)
	}

	return nil
}

// processUserConnectionClientSummaryMetrics processes user connection client summary metrics
func (s *UserConnectionScraper) processUserConnectionClientSummaryMetrics(result models.UserConnectionClientSummary, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Processing user connection client summary metrics")

	// Process unique hosts
	if result.UniqueHosts != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.unique_hosts")
		metric.SetDescription("Number of unique client hosts with connections")
		metric.SetUnit("hosts")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.UniqueHosts)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process unique programs
	if result.UniquePrograms != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.unique_programs")
		metric.SetDescription("Number of unique programs with connections")
		metric.SetUnit("programs")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.UniquePrograms)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process top host connection count
	if result.TopHostConnectionCount != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.top_host_connections")
		metric.SetDescription("Highest number of connections from a single host")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.TopHostConnectionCount)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process top program connection count
	if result.TopProgramConnectionCount != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.top_program_connections")
		metric.SetDescription("Highest number of connections from a single program")
		metric.SetUnit("connections")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.TopProgramConnectionCount)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process hosts with multiple programs
	if result.HostsWithMultiplePrograms != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.hosts_multi_program")
		metric.SetDescription("Number of hosts running multiple different programs")
		metric.SetUnit("hosts")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.HostsWithMultiplePrograms)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	// Process programs from multiple hosts
	if result.ProgramsFromMultipleHosts != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.client.programs_multi_host")
		metric.SetDescription("Number of programs connecting from multiple hosts")
		metric.SetUnit("programs")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.ProgramsFromMultipleHosts)

		attrs := dataPoint.Attributes()
		attrs.PutStr("source_type", "user_connections")
	}

	return nil
}

// processFailedLoginSummaryMetrics processes failed login summary metrics
func (s *UserConnectionScraper) processFailedLoginSummaryMetrics(result models.FailedLoginSummary, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Processing failed login summary metrics")

	// Helper function to add common attributes
	addCommonAttributes := func(attrs pcommon.Map) {
		attrs.PutStr("source_type", "user_connections")
		if result.Username != nil {
			attrs.PutStr("username", *result.Username)
		}
		if result.SourceIP != nil {
			attrs.PutStr("source_ip", *result.SourceIP)
		}
	}

	// Process total failed logins
	if result.TotalFailedLogins != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.total_failed_logins")
		metric.SetDescription("Total count of failed login attempts")
		metric.SetUnit("1")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.TotalFailedLogins)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process recent failed logins
	if result.RecentFailedLogins != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.recent_failed_logins")
		metric.SetDescription("Count of failed logins in the last hour")
		metric.SetUnit("1")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.RecentFailedLogins)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process unique failed users
	if result.UniqueFailedUsers != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.unique_failed_users")
		metric.SetDescription("Count of distinct usernames with failed logins")
		metric.SetUnit("1")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.UniqueFailedUsers)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	// Process unique failed sources
	if result.UniqueFailedSources != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.user_connections.authentication.unique_failed_sources")
		metric.SetDescription("Count of distinct source IPs with failed logins")
		metric.SetUnit("1")
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetIntValue(*result.UniqueFailedSources)

		attrs := dataPoint.Attributes()
		addCommonAttributes(attrs)
	}

	return nil
}
