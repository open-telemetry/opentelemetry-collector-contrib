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
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeInstanceBufferMetrics collects instance-level buffer pool metrics using engine-specific queries
func (s *InstanceScraper) ScrapeInstanceBufferMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server instance buffer metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.instance.buffer_pool_size")
	if !found {
		return fmt.Errorf("no buffer pool query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing buffer pool query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.InstanceBufferMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute instance buffer query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute instance buffer query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from instance buffer query - SQL Server may not be ready")
		return fmt.Errorf("no results returned from instance buffer query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from instance buffer query",
			zap.Int("result_count", len(results)))
	}

	result := results[0]
	if result.InstanceBufferPoolSize == nil {
		s.logger.Error("Instance buffer pool size is null - invalid query result")
		return fmt.Errorf("instance buffer pool size is null in query result")
	}

	if err := s.processInstanceBufferMetrics(result, scopeMetrics); err != nil {
		s.logger.Error("Failed to process instance buffer metrics", zap.Error(err))
		return fmt.Errorf("failed to process instance buffer metrics: %w", err)
	}

	s.logger.Debug("Successfully scraped instance buffer metrics",
		zap.Int64("buffer_pool_size", *result.InstanceBufferPoolSize))

	return nil
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

// processInstanceBufferMetrics processes buffer pool metrics and creates OpenTelemetry metrics
func (s *InstanceScraper) processInstanceBufferMetrics(result models.InstanceBufferMetrics, scopeMetrics pmetric.ScopeMetrics) error {
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

		if metricName == "" {
			continue
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetUnit("By")

		// Set description based on metric name
		switch metricName {
		case "sqlserver.buffer_pool.size_bytes":
			metric.SetDescription("Size of the SQL Server buffer pool in bytes")
		default:
			metric.SetDescription(fmt.Sprintf("SQL Server %s metric", metricName))
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
			s.logger.Info("Successfully scraped SQL Server instance buffer metric",
				zap.Int64("buffer_pool_size_bytes", value))
		case reflect.Float64:
			value := fieldValue.Float()
			dataPoint.SetDoubleValue(value)
			s.logger.Info("Successfully scraped SQL Server instance buffer metric",
				zap.Float64("value", value))
		case reflect.Int, reflect.Int32:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server instance buffer metric",
				zap.Int64("value", value))
		default:
			return fmt.Errorf("unsupported field type %s for metric %s", fieldValue.Kind(), metricName)
		}

		// Set attributes
		dataPoint.Attributes().PutStr("metric.type", sourceType)
		dataPoint.Attributes().PutStr("metric.source", "sys.dm_os_buffer_descriptors")
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	return nil
}
