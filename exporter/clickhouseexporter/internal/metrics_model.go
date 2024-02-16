// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var supportedMetricTypes = map[string]struct{}{
	createGaugeTableSQL:        {},
	createSumTableSQL:          {},
	createHistogramTableSQL:    {},
	createExpHistogramTableSQL: {},
	createSummaryTableSQL:      {},
}

var logger *zap.Logger

// MetricsModel is used to group metric data and insert into clickhouse
// any type of metrics need implement it.
type MetricsModel interface {
	// Add used to bind MetricsMetaData to a specific metric then put them into a slice
	Add(resAttr map[string]string, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error
	// insert is used to insert metric data to clickhouse
	insert(ctx context.Context, db *sql.DB) error
}

// MetricsMetaData  contain specific metric data
type MetricsMetaData struct {
	ResAttr    map[string]string
	ResURL     string
	ScopeURL   string
	ScopeInstr pcommon.InstrumentationScope
}

// SetLogger set a logger instance
func SetLogger(l *zap.Logger) {
	logger = l
}

// NewMetricsTable create metric tables with an expiry time to storage metric telemetry data
func NewMetricsTable(ctx context.Context, tableName string, ttlExpr string, db *sql.DB) error {
	for table := range supportedMetricTypes {
		query := fmt.Sprintf(table, tableName, ttlExpr)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("exec create metrics table sql: %w", err)
		}
	}
	return nil
}

// NewMetricsModel create a model for contain different metric data
func NewMetricsModel(tableName string) map[pmetric.MetricType]MetricsModel {
	return map[pmetric.MetricType]MetricsModel{
		pmetric.MetricTypeGauge: &gaugeMetrics{
			insertSQL: fmt.Sprintf(insertGaugeTableSQL, tableName),
		},
		pmetric.MetricTypeSum: &sumMetrics{
			insertSQL: fmt.Sprintf(insertSumTableSQL, tableName),
		},
		pmetric.MetricTypeHistogram: &histogramMetrics{
			insertSQL: fmt.Sprintf(insertHistogramTableSQL, tableName),
		},
		pmetric.MetricTypeExponentialHistogram: &expHistogramMetrics{
			insertSQL: fmt.Sprintf(insertExpHistogramTableSQL, tableName),
		},
		pmetric.MetricTypeSummary: &summaryMetrics{
			insertSQL: fmt.Sprintf(insertSummaryTableSQL, tableName),
		},
	}
}

// InsertMetrics insert metric data into clickhouse concurrently
func InsertMetrics(ctx context.Context, db *sql.DB, metricsMap map[pmetric.MetricType]MetricsModel) error {
	errsChan := make(chan error, len(supportedMetricTypes))
	wg := &sync.WaitGroup{}
	for _, m := range metricsMap {
		wg.Add(1)
		go func(m MetricsModel, wg *sync.WaitGroup) {
			errsChan <- m.insert(ctx, db)
			wg.Done()
		}(m, wg)
	}
	wg.Wait()
	close(errsChan)
	var errs error
	for err := range errsChan {
		errs = errors.Join(errs, err)
	}
	return errs
}

func convertExemplars(exemplars pmetric.ExemplarSlice) (clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet) {
	var (
		attrs    clickhouse.ArraySet
		times    clickhouse.ArraySet
		values   clickhouse.ArraySet
		traceIDs clickhouse.ArraySet
		spanIDs  clickhouse.ArraySet
	)
	for i := 0; i < exemplars.Len(); i++ {
		exemplar := exemplars.At(i)
		attrs = append(attrs, attributesToMap(exemplar.FilteredAttributes()))
		times = append(times, exemplar.Timestamp().AsTime())
		values = append(values, getValue(exemplar.IntValue(), exemplar.DoubleValue(), exemplar.ValueType()))

		traceID, spanID := exemplar.TraceID(), exemplar.SpanID()
		traceIDs = append(traceIDs, hex.EncodeToString(traceID[:]))
		spanIDs = append(spanIDs, hex.EncodeToString(spanID[:]))
	}
	return attrs, times, values, traceIDs, spanIDs
}

// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto#L358
// define two types for one datapoint value, clickhouse only use one value of float64 to store them
func getValue(intValue int64, floatValue float64, dataType any) float64 {
	switch t := dataType.(type) {
	case pmetric.ExemplarValueType:
		switch t {
		case pmetric.ExemplarValueTypeDouble:
			return floatValue
		case pmetric.ExemplarValueTypeInt:
			return float64(intValue)
		case pmetric.ExemplarValueTypeEmpty:
			logger.Warn("Examplar value type is unset, use 0.0 as default")
			return 0.0
		default:
			logger.Warn("Can't find a suitable value for ExemplarValueType, use 0.0 as default")
			return 0.0
		}
	case pmetric.NumberDataPointValueType:
		switch t {
		case pmetric.NumberDataPointValueTypeDouble:
			return floatValue
		case pmetric.NumberDataPointValueTypeInt:
			return float64(intValue)
		case pmetric.NumberDataPointValueTypeEmpty:
			logger.Warn("DataPoint value type is unset, use 0.0 as default")
			return 0.0
		default:
			logger.Warn("Can't find a suitable value for NumberDataPointValueType, use 0.0 as default")
			return 0.0
		}
	default:
		logger.Warn("unsupported ValueType, current support: ExemplarValueType, NumberDataPointValueType, ues 0.0 as default")
		return 0.0
	}
}

func attributesToMap(attributes pcommon.Map) map[string]string {
	m := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})
	return m
}

func convertSliceToArraySet[T any](slice []T) clickhouse.ArraySet {
	var set clickhouse.ArraySet
	for _, item := range slice {
		set = append(set, item)
	}
	return set
}

func convertValueAtQuantile(valueAtQuantile pmetric.SummaryDataPointValueAtQuantileSlice) (clickhouse.ArraySet, clickhouse.ArraySet) {
	var (
		quantiles clickhouse.ArraySet
		values    clickhouse.ArraySet
	)
	for i := 0; i < valueAtQuantile.Len(); i++ {
		value := valueAtQuantile.At(i)
		quantiles = append(quantiles, value.Quantile())
		values = append(values, value.Value())
	}
	return quantiles, values
}

// doWithTx is a copy of clickhouseexporter.doWithTx, it starts a transaction to exec SQL in fn.
// This function is in a temporary status, after this PR get merged,
// there will be a PR to move all db function and tool function to internal package.
func doWithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func newPlaceholder(count int) *string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		b.WriteString(",?")
	}
	b.WriteString("),")
	placeholder := strings.Replace(b.String(), ",", "(", 1)
	return &placeholder
}
