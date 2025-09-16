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
	connection SQLConnectionInterface
	logger     *zap.Logger
	startTime  pcommon.Timestamp
}

// NewInstanceScraper creates a new instance scraper
func NewInstanceScraper(conn SQLConnectionInterface, logger *zap.Logger) *InstanceScraper {
	return &InstanceScraper{
		connection: conn,
		logger:     logger,
		startTime:  pcommon.NewTimestampFromTime(time.Now()),
	}
}

// ScrapeInstanceBufferMetrics collects instance-level buffer pool metrics
func (s *InstanceScraper) ScrapeInstanceBufferMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server instance buffer metrics")

	var results []models.InstanceBufferMetrics
	if err := s.connection.Query(ctx, &results, queries.InstanceBufferPoolQuery); err != nil {
		s.logger.Error("Failed to execute instance buffer query",
			zap.Error(err),
			zap.String("query", queries.InstanceBufferPoolQuery))
		return fmt.Errorf("failed to execute instance buffer query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from instance buffer query - SQL Server may not be ready")
		return fmt.Errorf("no results returned from instance buffer query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from instance buffer query",
			zap.Int("result_count", len(results)))
		// Use the first result but log the issue
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
		if err := s.createMetric(field, metricName, sourceType, scopeMetrics); err != nil {
			return fmt.Errorf("failed to create metric %s: %w", metricName, err)
		}
	}

	return nil
}

// createMetric creates an OpenTelemetry metric from the field value
func (s *InstanceScraper) createMetric(field reflect.Value, metricName, sourceType string, scopeMetrics pmetric.ScopeMetrics) error {
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

	// Set the value
	if err := s.setDataPointValue(field, dataPoint); err != nil {
		return err
	}

	// Set attributes
	dataPoint.Attributes().PutStr("metric.type", sourceType)
	dataPoint.Attributes().PutStr("metric.source", "sys.dm_os_buffer_descriptors")

	return nil
}

// setDataPointValue sets the value on a data point based on the field type
func (s *InstanceScraper) setDataPointValue(field reflect.Value, dataPoint pmetric.NumberDataPoint) error {
	// Handle pointer fields
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return fmt.Errorf("field value is nil")
		}
		field = field.Elem()
	}

	switch field.Kind() {
	case reflect.Int64:
		value := field.Int()
		dataPoint.SetIntValue(value)
		s.logger.Info("Successfully scraped SQL Server instance buffer metric",
			zap.Int64("buffer_pool_size_bytes", value))
	case reflect.Float64:
		value := field.Float()
		dataPoint.SetDoubleValue(value)
		s.logger.Info("Successfully scraped SQL Server instance buffer metric",
			zap.Float64("value", value))
	case reflect.Int, reflect.Int32:
		value := field.Int()
		dataPoint.SetIntValue(value)
		s.logger.Info("Successfully scraped SQL Server instance buffer metric",
			zap.Int64("value", value))
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}
