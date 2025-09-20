// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// rateTracker tracks previous values for rate calculation
type rateTracker struct {
	previousValues map[string]float64
	previousTime   map[string]time.Time
	mutex          sync.RWMutex
}

// newRateTracker creates a new rate tracker
func newRateTracker() *rateTracker {
	return &rateTracker{
		previousValues: make(map[string]float64),
		previousTime:   make(map[string]time.Time),
	}
}

// calculateRate calculates the rate from current and previous values
func (rt *rateTracker) calculateRate(metricName string, currentValue float64, currentTime time.Time) (float64, bool) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	previousValue, hasPrevious := rt.previousValues[metricName]
	previousTime, hasTime := rt.previousTime[metricName]

	// Store current values for next calculation
	rt.previousValues[metricName] = currentValue
	rt.previousTime[metricName] = currentTime

	// Return rate only if we have previous values
	if !hasPrevious || !hasTime {
		return 0, false // First collection, no rate available
	}

	timeDelta := currentTime.Sub(previousTime).Seconds()
	if timeDelta <= 0 {
		return 0, false
	}

	valueDelta := currentValue - previousValue
	if valueDelta < 0 {
		// Counter reset, skip this calculation
		return 0, false
	}

	return valueDelta / timeDelta, true
}

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
	rateTracker   *rateTracker
}

// NewInstanceScraper creates a new instance scraper
func NewInstanceScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *InstanceScraper {
	return &InstanceScraper{
		connection:    conn,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
		rateTracker:   newRateTracker(),
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

		// Process rate metrics vs gauge metrics differently
		var finalValue float64
		var shouldEmitMetric bool = true

		if sourceType == "rate" {
			// For rate metrics, calculate delta from cumulative counter values
			currentTime := time.Now()
			calculatedRate, hasRate := s.rateTracker.calculateRate(metricName, rawValue, currentTime)
			if hasRate {
				finalValue = calculatedRate
				s.logger.Debug("Calculated rate for SQL Server metric",
					zap.String("metric_name", metricName),
					zap.Float64("raw_cumulative_value", rawValue),
					zap.Float64("calculated_rate", calculatedRate))
			} else {
				// First collection - no previous value to calculate rate
				shouldEmitMetric = false
				s.logger.Debug("Skipping first collection for rate metric (no previous value)",
					zap.String("metric_name", metricName),
					zap.Float64("raw_cumulative_value", rawValue))
			}
		} else {
			// For gauge metrics, use the raw value directly
			finalValue = rawValue
		}

		// Only emit the metric if we have a valid value
		if !shouldEmitMetric {
			continue
		}

		// Create appropriate OpenTelemetry metric type
		var dataPoint pmetric.NumberDataPoint
		if sourceType == "rate" {
			// Rate metrics are sent as gauges (instantaneous rate values)
			gauge := metric.SetEmptyGauge()
			dataPoint = gauge.DataPoints().AppendEmpty()
		} else {
			// Gauge metrics
			gauge := metric.SetEmptyGauge()
			dataPoint = gauge.DataPoints().AppendEmpty()
		}

		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Set the final calculated value
		dataPoint.SetDoubleValue(finalValue)

		s.logger.Debug("Successfully scraped SQL Server instance stat metric",
			zap.String("metric_name", metricName),
			zap.String("source_type", sourceType),
			zap.Float64("final_value", finalValue))

		// Set attributes
		dataPoint.Attributes().PutStr("metric.type", sourceType)
	}

	return nil
}
