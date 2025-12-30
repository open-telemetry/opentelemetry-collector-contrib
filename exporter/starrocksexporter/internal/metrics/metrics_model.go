// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metrics"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/sqltemplates"
)

var supportedMetricTypes = map[pmetric.MetricType]string{
	pmetric.MetricTypeGauge:                sqltemplates.MetricsGaugeCreateTable,
	pmetric.MetricTypeSum:                  sqltemplates.MetricsSumCreateTable,
	pmetric.MetricTypeHistogram:            sqltemplates.MetricsHistogramCreateTable,
	pmetric.MetricTypeExponentialHistogram: sqltemplates.MetricsExpHistogramCreateTable,
	pmetric.MetricTypeSummary:              sqltemplates.MetricsSummaryCreateTable,
}

var logger *zap.Logger

type MetricTablesConfigMapper map[pmetric.MetricType]MetricTypeConfig

type MetricTypeConfig struct {
	Name string `mapstructure:"name"`
}

// MetricsModel is used to group metric data and insert into StarRocks
// any type of metrics need implement it.
type MetricsModel interface {
	// Add used to bind MetricsMetaData to a specific metric then put them into a slice
	Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric)

	// insert is used to insert metric data to StarRocks
	insert(ctx context.Context, db *sql.DB) error
}

// MetricsMetaData contain specific metric data
type MetricsMetaData struct {
	ResAttr    pcommon.Map
	ResURL     string
	ScopeURL   string
	ScopeInstr pcommon.InstrumentationScope
}

// SetLogger set a logger instance
func SetLogger(l *zap.Logger) {
	logger = l
}

// NewMetricsTable create metric tables
func NewMetricsTable(ctx context.Context, tablesConfig MetricTablesConfigMapper, database string, db *sql.DB) error {
	for key, ddlTemplate := range supportedMetricTypes {
		query := fmt.Sprintf(ddlTemplate, database, tablesConfig[key].Name)
		_, err := db.ExecContext(ctx, query)
		if err != nil {
			return fmt.Errorf("exec create metrics table sql: %w", err)
		}
	}
	return nil
}

// NewMetricsModel create a model for contain different metric data
func NewMetricsModel(tablesConfig MetricTablesConfigMapper, database string) map[pmetric.MetricType]MetricsModel {
	return map[pmetric.MetricType]MetricsModel{
		pmetric.MetricTypeGauge: &gaugeMetrics{
			insertSQL: fmt.Sprintf(sqltemplates.MetricsGaugeInsert, database, tablesConfig[pmetric.MetricTypeGauge].Name),
		},
		pmetric.MetricTypeSum: &sumMetrics{
			insertSQL: fmt.Sprintf(sqltemplates.MetricsSumInsert, database, tablesConfig[pmetric.MetricTypeSum].Name),
		},
		pmetric.MetricTypeHistogram: &histogramMetrics{
			insertSQL: fmt.Sprintf(sqltemplates.MetricsHistogramInsert, database, tablesConfig[pmetric.MetricTypeHistogram].Name),
		},
		pmetric.MetricTypeExponentialHistogram: &expHistogramMetrics{
			insertSQL: fmt.Sprintf(sqltemplates.MetricsExpHistogramInsert, database, tablesConfig[pmetric.MetricTypeExponentialHistogram].Name),
		},
		pmetric.MetricTypeSummary: &summaryMetrics{
			insertSQL: fmt.Sprintf(sqltemplates.MetricsSummaryInsert, database, tablesConfig[pmetric.MetricTypeSummary].Name),
		},
	}
}

// InsertMetrics insert metric data into StarRocks concurrently
func InsertMetrics(ctx context.Context, db *sql.DB, metricsMap map[pmetric.MetricType]MetricsModel) error {
	errsChan := make(chan error, len(supportedMetricTypes))
	wg := &sync.WaitGroup{}
	for _, m := range metricsMap {
		wg.Add(1)
		go func(model MetricsModel) {
			defer wg.Done()
			if err := model.insert(ctx, db); err != nil {
				errsChan <- err
			}
		}(m)
	}
	wg.Wait()
	close(errsChan)

	var errs []error
	for err := range errsChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
