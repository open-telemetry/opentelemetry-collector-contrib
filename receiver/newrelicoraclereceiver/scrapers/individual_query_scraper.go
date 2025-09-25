// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// InstanceInfo represents Oracle instance information for resource attributes
type InstanceInfo struct {
	InstanceID   string
	InstanceName string
	GlobalName   string
	DbID         string
}

// SetResourceAttributes sets resource attributes based on instance information
func (i *InstanceInfo) SetResourceAttributes(res pcommon.Resource) {
	res.Attributes().PutStr("newrelic.oracle.instance.id", i.InstanceID)
	res.Attributes().PutStr("newrelic.oracle.instance.name", i.InstanceName)
	res.Attributes().PutStr("newrelic.oracle.global.name", i.GlobalName)
	res.Attributes().PutStr("newrelic.oracle.db.id", i.DbID)
}

// IndividualQueryScraper handles Oracle individual query metrics collection
type IndividualQueryScraper struct {
	logger *zap.Logger
}

// NewIndividualQueryScraper creates a new individual query scraper
func NewIndividualQueryScraper(logger *zap.Logger) *IndividualQueryScraper {
	return &IndividualQueryScraper{
		logger: logger,
	}
}

// IndividualQueryConfig represents individual query filtering configuration (imported from queries package)
type IndividualQueryConfig = queries.IndividualQueryConfig

// CollectIndividualQueryMetrics collects metrics related to Oracle database individual queries.
// It returns a pmetric.Metrics object containing a single metric (OracleIndividualQuerySample)
// with all individual query attributes as one data point per query.
func (s *IndividualQueryScraper) CollectIndividualQueryMetrics(db *sql.DB, skipGroups []string, instanceInfo *InstanceInfo, config *IndividualQueryConfig) (pmetric.Metrics, error) {
	// Skip collection if individual query metrics are in the skip list
	for _, skipGroup := range skipGroups {
		if skipGroup == "individual_query_metrics" {
			s.logger.Info("Skipping individual query metrics collection as configured in skip_metrics_groups")
			return pmetric.NewMetrics(), nil
		}
	}

	// Skip if individual queries are disabled in configuration
	if config != nil && !config.Enabled {
		s.logger.Info("Individual query metrics collection is disabled in configuration")
		return pmetric.NewMetrics(), nil
	}

	// Use default config if none provided
	defaultConfig := IndividualQueryConfig{
		Enabled:        true,
		SearchText:     "SELECT", // Default search text
		ExcludeSchemas: []string{},
		MaxQueries:     10,
	}
	if config == nil {
		config = &defaultConfig
	}

	// Build dynamic SQL based on configuration
	individualQuerySQL := queries.BuildIndividualQuerySQL(*config)
	s.logger.Debug("Executing individual query metrics SQL",
		zap.String("query", individualQuerySQL),
		zap.String("search_text", config.SearchText),
		zap.Strings("exclude_schemas", config.ExcludeSchemas),
		zap.Int("max_queries", config.MaxQueries))

	rows, err := db.Query(individualQuerySQL)
	if err != nil {
		s.logger.Error("Failed to execute individual query metrics SQL", zap.Error(err))
		return pmetric.NewMetrics(), fmt.Errorf("error collecting individual query metrics: %w", err)
	}
	s.logger.Debug("Successfully executed individual query metrics SQL")
	defer rows.Close()

	now := pcommon.NewTimestampFromTime(time.Now())

	// Create a single metric for all individual query samples
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes from instance info
	if instanceInfo != nil {
		instanceInfo.SetResourceAttributes(rm.Resource())
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver")
	sm.Scope().SetVersion("0.1.0")

	individualQueryCount := 0

	sampleMetric := sm.Metrics().AppendEmpty()
	sampleMetric.SetName("OracleIndividualQuerySample")
	sampleMetric.SetDescription("Sample of Oracle individual query with all metrics as attributes")
	sampleMetric.SetUnit("ms")
	sampleGauge := sampleMetric.SetEmptyGauge()

	// Process query results
	for rows.Next() {
		var sqlID, sqlFullText string
		var childNumber, executions int64
		var elapsedTimeMs float64

		if err := rows.Scan(&sqlID, &childNumber, &sqlFullText, &executions, &elapsedTimeMs); err != nil {
			s.logger.Warn("Error scanning individual query metrics row", zap.Error(err))
			continue
		}

		// Skip queries with invalid metrics
		if elapsedTimeMs < 0 || executions <= 0 {
			s.logger.Warn("Skipping individual query with invalid metrics",
				zap.String("sql_id", sqlID),
				zap.Float64("elapsed_time_ms", elapsedTimeMs),
				zap.Int64("executions", executions))
			continue
		}

		dp := sampleGauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetDoubleValue(elapsedTimeMs)

		// Set attributes for the individual query
		dp.Attributes().PutStr("sql_id", sqlID)
		dp.Attributes().PutInt("child_number", childNumber)
		dp.Attributes().PutStr("sql_fulltext", sqlFullText)
		dp.Attributes().PutInt("executions", executions)
		dp.Attributes().PutDouble("elapsed_time_ms", elapsedTimeMs)
		dp.Attributes().PutStr("search_text", config.SearchText)

		individualQueryCount++
	}

	s.logger.Info("OracleIndividualQuerySample metrics collection completed",
		zap.Int("total_individual_queries", individualQueryCount),
		zap.String("search_text", config.SearchText))

	return metrics, nil
}
