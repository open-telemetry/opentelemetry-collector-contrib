// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package scrapers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// SQLConnectionInterface defines the interface for database connections
type SQLConnectionInterface interface {
	Query(ctx context.Context, dest interface{}, query string) error
}

// InstanceScraper handles SQL Server instance-level metrics collection
type InstanceScraper struct {
	connection    SQLConnectionInterface
	logger        *zap.Logger
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewInstanceScraper creates a new instance scraper
func NewInstanceScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *InstanceScraper {
	return &InstanceScraper{
		connection:    conn,
		logger:        logger,
		engineEdition: engineEdition,
	}
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition with Default fallback
func (s *InstanceScraper) getQueryForMetric(metricName string) (string, bool) {
	query, found := queries.GetQueryForMetric(queries.InstanceQueries, metricName, s.engineEdition)
	if found {
		s.logger.Debug("Using query for metric",
			zap.String("metric_name", metricName),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
	}
	return query, found
}

func (s *InstanceScraper) ScrapeInstanceMemoryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server instance memory metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.instance.memory_metrics")
	if !found {
		return fmt.Errorf("no memory query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing memory query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.InstanceMemoryDefinitionsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute instance memory query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute instance memory query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from instance memory query - SQL Server may not be ready")
		return fmt.Errorf("no results returned from instance memory query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from instance memory query",
			zap.Int("result_count", len(results)))
	}

	result := results[0]
	if result.TotalPhysicalMemory == nil && result.AvailablePhysicalMemory == nil && result.MemoryUtilization == nil {
		s.logger.Error("All memory metrics are null - invalid query result")
		return fmt.Errorf("all memory metrics are null in query result")
	}

	if err := s.processInstanceMemoryMetrics(result, scopeMetrics); err != nil {
		s.logger.Error("Failed to process instance memory metrics", zap.Error(err))
		return fmt.Errorf("failed to process instance memory metrics: %w", err)
	}

	s.logger.Debug("Successfully scraped instance memory metrics")
	return nil
}

// ScrapeInstanceComprehensiveStats scrapes comprehensive instance statistics
func (s *InstanceScraper) ScrapeInstanceComprehensiveStats(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server comprehensive instance statistics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.instance.comprehensive_stats")
	if !found {
		return fmt.Errorf("no comprehensive stats query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing comprehensive stats query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.InstanceStatsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute comprehensive stats query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute comprehensive stats query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from comprehensive stats query - SQL Server may not be ready")
		return fmt.Errorf("no results returned from comprehensive stats query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from comprehensive stats query",
			zap.Int("result_count", len(results)))
	}

	result := results[0]
	if err := s.processInstanceStatsMetrics(result, scopeMetrics); err != nil {
		s.logger.Error("Failed to process comprehensive stats metrics", zap.Error(err))
		return fmt.Errorf("failed to process comprehensive stats metrics: %w", err)
	}

	s.logger.Debug("Successfully scraped comprehensive instance statistics")
	return nil
}

// processInstanceMemoryMetrics processes memory metrics and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceMemoryMetrics(result models.InstanceMemoryDefinitionsModel, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")

		if metricName == "" {
			continue
		}

		// Create the metric using the metric_name from struct tag
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)

		// Set description from tag or generate default
		if description != "" {
			metric.SetDescription(description)
		} else {
			// Generate default description based on metric name
			switch metricName {
			case "memoryTotal":
				metric.SetDescription("Total physical memory available to SQL Server")
			case "memoryAvailable":
				metric.SetDescription("Available physical memory")
			case "memoryUtilization":
				metric.SetDescription("Memory utilization percentage")
			default:
				metric.SetDescription(fmt.Sprintf("SQL Server %s metric", metricName))
			}
		}

		// Set unit from tag or generate default
		if unit != "" {
			metric.SetUnit(unit)
		} else {
			// Generate default unit based on metric name
			if metricName == "memoryUtilization" {
				metric.SetUnit("Percent")
			} else {
				metric.SetUnit("By")
			}
		}

		// Create gauge metric (for instance metrics)
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Handle pointer fields and set the value
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return fmt.Errorf("field value is nil for metric %s", metricName)
			}
			fieldValue = field.Elem()
		}

		// Set the value based on field type
		switch fieldValue.Kind() {
		case reflect.Int64:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server instance memory metric",
				zap.String("metric_name", metricName),
				zap.Int64("value", value))
		case reflect.Float64:
			value := fieldValue.Float()
			dataPoint.SetDoubleValue(value)
			s.logger.Info("Successfully scraped SQL Server instance memory metric",
				zap.String("metric_name", metricName),
				zap.Float64("value", value))
		case reflect.Int, reflect.Int32:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server instance memory metric",
				zap.String("metric_name", metricName),
				zap.Int64("value", value))
		default:
			return fmt.Errorf("unsupported field type %s for metric %s", fieldValue.Kind(), metricName)
		}

		// Set attributes
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}

	return nil
}

// ScrapeInstanceStats scrapes comprehensive SQL Server instance statistics
func (s *InstanceScraper) ScrapeInstanceStats(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server comprehensive instance statistics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.instance.comprehensive_stats")
	if !found {
		return fmt.Errorf("no comprehensive stats query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing comprehensive stats query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.InstanceStatsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute instance stats query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute instance stats query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from instance stats query - SQL Server may not be ready")
		return fmt.Errorf("no results returned from instance stats query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from instance stats query",
			zap.Int("result_count", len(results)))
	}

	result := results[0]
	if err := s.processInstanceStatsMetrics(result, scopeMetrics); err != nil {
		s.logger.Error("Failed to process instance stats metrics", zap.Error(err))
		return fmt.Errorf("failed to process instance stats metrics: %w", err)
	}

	s.logger.Debug("Successfully scraped comprehensive instance statistics")
	return nil
}

// processInstanceStatsMetrics processes comprehensive instance statistics and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceStatsMetrics(result models.InstanceStatsModel, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values
		if field.Kind() == reflect.Ptr && field.IsNil() {
			s.logger.Debug("Skipping nil field", zap.String("field_name", fieldType.Name))
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")

		if metricName == "" {
			continue
		}

		// Create the metric using the metric_name from struct tag
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)

		// Set description from tag
		if description != "" {
			metric.SetDescription(description)
		}

		// Set unit from tag
		if unit != "" {
			metric.SetUnit(unit)
		}

		// Handle pointer fields to get the raw value
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				s.logger.Debug("Field value is nil for metric", zap.String("metric_name", metricName))
				continue
			}
			fieldValue = field.Elem()
		}

		// Get the raw value from SQL Server
		var rawValue float64
		switch fieldValue.Kind() {
		case reflect.Int64:
			rawValue = float64(fieldValue.Int())
		case reflect.Float64:
			rawValue = fieldValue.Float()
		case reflect.Int, reflect.Int32:
			rawValue = float64(fieldValue.Int())
		default:
			return fmt.Errorf("unsupported field type %s for metric %s", fieldValue.Kind(), metricName)
		}

		// Create appropriate OpenTelemetry metric type based on source_type
		var dataPoint pmetric.NumberDataPoint
		if sourceType == "rate" {
			sum := metric.SetEmptySum()
			sum.SetIsMonotonic(true)
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			dataPoint = sum.DataPoints().AppendEmpty()
		} else {
			// Gauge metrics for instantaneous values
			gauge := metric.SetEmptyGauge()
			dataPoint = gauge.DataPoints().AppendEmpty()
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		dataPoint.SetTimestamp(now)

		if sourceType == "rate" {
			// For cumulative metrics, the start time is fixed to the scraper's initialization time.
			dataPoint.SetStartTimestamp(s.startTime)
		} else {
			// For gauge metrics, the interval is instantaneous, so start time is the same as the timestamp.
			dataPoint.SetStartTimestamp(now)
		}

		// Set the raw cumulative value from SQL Server (for rate metrics, this will be converted to delta downstream)
		dataPoint.SetDoubleValue(rawValue)

		s.logger.Debug("Successfully scraped SQL Server instance stat metric",
			zap.String("metric_name", metricName),
			zap.String("source_type", sourceType),
			zap.Float64("raw_value", rawValue))

		// Set attributes
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}

	return nil
}

// ScrapeInstanceBufferPoolHitPercent scrapes buffer pool hit percent metric
func (s *InstanceScraper) ScrapeInstanceBufferPoolHitPercent(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.buffer_pool.hitPercent")
	if !found {
		return fmt.Errorf("no buffer pool hit percent query available for engine edition %d", s.engineEdition)
	}
	var results []models.BufferPoolHitPercentMetricsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute buffer pool hit percent query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from buffer pool hit percent query")
	}
	return s.processInstanceBufferPoolHitPercent(results[0], scopeMetrics)
}

// processInstanceBufferPoolHitPercent processes buffer pool hit percent metric and creates OpenTelemetry metric
func (s *InstanceScraper) processInstanceBufferPoolHitPercent(result models.BufferPoolHitPercentMetricsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Float64 {
			dataPoint.SetDoubleValue(fieldValue.Float())
		} else if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}

// ScrapeInstanceProcessCounts scrapes process counts
func (s *InstanceScraper) ScrapeInstanceProcessCounts(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.process_counts")
	if !found {
		return fmt.Errorf("no process counts query available for engine edition %d", s.engineEdition)
	}
	var results []models.InstanceProcessCountsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute process counts query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from process counts query")
	}
	return s.processInstanceProcessCounts(results[0], scopeMetrics)
}

// processInstanceProcessCounts processes process counts and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceProcessCounts(result models.InstanceProcessCountsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}

// ScrapeInstanceRunnableTasks scrapes runnable tasks
func (s *InstanceScraper) ScrapeInstanceRunnableTasks(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.runnable_tasks")
	if !found {
		return fmt.Errorf("no runnable tasks query available for engine edition %d", s.engineEdition)
	}
	var results []models.RunnableTasksMetricsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute runnable tasks query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from runnable tasks query")
	}
	return s.processInstanceRunnableTasks(results[0], scopeMetrics)
}

// processInstanceRunnableTasks processes runnable tasks and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceRunnableTasks(result models.RunnableTasksMetricsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}

// ScrapeInstanceDiskMetrics scrapes disk metrics
func (s *InstanceScraper) ScrapeInstanceDiskMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.disk_metrics")
	if !found {
		return fmt.Errorf("no disk metrics query available for engine edition %d", s.engineEdition)
	}
	var results []models.InstanceDiskMetricsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute disk metrics query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from disk metrics query")
	}
	return s.processInstanceDiskMetrics(results[0], scopeMetrics)
}

// processInstanceDiskMetrics processes disk metrics and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceDiskMetrics(result models.InstanceDiskMetricsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}

// ScrapeInstanceActiveConnections scrapes active connections
func (s *InstanceScraper) ScrapeInstanceActiveConnections(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.active_connections")
	if !found {
		return fmt.Errorf("no active connections query available for engine edition %d", s.engineEdition)
	}
	var results []models.InstanceActiveConnectionsMetricsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute active connections query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from active connections query")
	}
	return s.processInstanceActiveConnections(results[0], scopeMetrics)
}

// processInstanceActiveConnections processes active connections and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceActiveConnections(result models.InstanceActiveConnectionsMetricsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}

// ScrapeInstanceBufferPoolSize scrapes buffer pool size
func (s *InstanceScraper) ScrapeInstanceBufferPoolSize(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	query, found := s.getQueryForMetric("sqlserver.instance.buffer_pool_size")
	if !found {
		return fmt.Errorf("no buffer pool size query available for engine edition %d", s.engineEdition)
	}
	var results []models.InstanceBufferMetricsModel
	if err := s.connection.Query(ctx, &results, query); err != nil {
		return fmt.Errorf("failed to execute buffer pool size query: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned from buffer pool size query")
	}
	return s.processInstanceBufferPoolSize(results[0], scopeMetrics)
}

// processInstanceBufferPoolSize processes buffer pool size and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceBufferPoolSize(result models.InstanceBufferMetricsModel, scopeMetrics pmetric.ScopeMetrics) error {
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)
	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")
		description := fieldType.Tag.Get("description")
		unit := fieldType.Tag.Get("unit")
		if metricName == "" {
			continue
		}
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		if description != "" {
			metric.SetDescription(description)
		}
		if unit != "" {
			metric.SetUnit(unit)
		}
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			fieldValue = field.Elem()
		}
		if fieldValue.Kind() == reflect.Int64 {
			dataPoint.SetIntValue(fieldValue.Int())
		}
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}
	return nil
}
