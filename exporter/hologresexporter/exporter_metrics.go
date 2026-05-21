// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Column lists for COPY into each metric type table (must match DDL in exporter_common.go).
var (
	gaugeColumns = []string{
		"timestamp", "metric_name", "service_name", "value", "flags",
		"resource_attributes", "scope_name", "scope_version", "scope_attributes", "attributes",
	}
	sumColumns = []string{
		"timestamp", "start_timestamp", "metric_name", "service_name", "value", "flags",
		"is_monotonic", "aggregation_temporality",
		"resource_attributes", "scope_name", "scope_version", "scope_attributes", "attributes",
	}
	histogramColumns = []string{
		"timestamp", "start_timestamp", "metric_name", "service_name",
		"count", "sum", "min", "max", "flags",
		"bucket_counts", "explicit_bounds", "aggregation_temporality",
		"resource_attributes", "scope_name", "scope_version", "scope_attributes", "attributes",
	}
	summaryColumns = []string{
		"timestamp", "start_timestamp", "metric_name", "service_name",
		"count", "sum", "flags", "quantile_values", "quantile_counts",
		"resource_attributes", "scope_name", "scope_version", "scope_attributes", "attributes",
	}
	expHistogramColumns = []string{
		"timestamp", "start_timestamp", "metric_name", "service_name",
		"count", "sum", "min", "max", "scale", "zero_count", "flags",
		"positive_offset", "positive_bucket_counts", "negative_offset", "negative_bucket_counts",
		"aggregation_temporality",
		"resource_attributes", "scope_name", "scope_version", "scope_attributes", "attributes",
	}
)

// metricsBatch collects rows for a single metric type table and writes them
// using the PostgreSQL COPY protocol via pgx's CopyFrom.
type metricsBatch struct {
	tableName string
	columns   []string
	rows      [][]any
}

func (b *metricsBatch) addRow(args ...any) {
	b.rows = append(b.rows, args)
}

func (b *metricsBatch) insert(ctx context.Context, db pgxDB) error {
	if len(b.rows) == 0 {
		return nil
	}

	if _, err := db.CopyFrom(
		ctx,
		pgx.Identifier{b.tableName},
		b.columns,
		pgx.CopyFromRows(b.rows),
	); err != nil {
		return fmt.Errorf("failed to copy metrics into %s: %w", b.tableName, err)
	}
	return nil
}

type metricsExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     pgxDB
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) *metricsExporter {
	return &metricsExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(ctx, e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createMetricsTables(ctx, e.db, e.cfg.MetricsTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		e.db.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricData(ctx context.Context, md pmetric.Metrics) error {
	start := time.Now()

	gaugeData := &metricsBatch{
		tableName: e.cfg.MetricsTableName + "_gauge",
		columns:   gaugeColumns,
	}
	sumData := &metricsBatch{
		tableName: e.cfg.MetricsTableName + "_sum",
		columns:   sumColumns,
	}
	histogramData := &metricsBatch{
		tableName: e.cfg.MetricsTableName + "_histogram",
		columns:   histogramColumns,
	}
	summaryData := &metricsBatch{
		tableName: e.cfg.MetricsTableName + "_summary",
		columns:   summaryColumns,
	}
	expHistData := &metricsBatch{
		tableName: e.cfg.MetricsTableName + "_exp_histogram",
		columns:   expHistogramColumns,
	}

	rmSlice := md.ResourceMetrics()
	for i := range rmSlice.Len() {
		rm := rmSlice.At(i)
		serviceName := getServiceName(rm.Resource())
		resourceAttrs, err := attributesToJSON(rm.Resource().Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal resource attributes: %w", err)
		}

		smSlice := rm.ScopeMetrics()
		for j := range smSlice.Len() {
			sm := smSlice.At(j)
			scopeName := sm.Scope().Name()
			scopeVersion := sm.Scope().Version()
			scopeAttrs, err := attributesToJSON(sm.Scope().Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal scope attributes: %w", err)
			}

			metrics := sm.Metrics()
			for k := range metrics.Len() {
				metric := metrics.At(k)
				metricName := metric.Name()

				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					e.collectGauge(metric.Gauge(), metricName, serviceName, resourceAttrs, scopeName, scopeVersion, scopeAttrs, gaugeData)

				case pmetric.MetricTypeSum:
					e.collectSum(metric.Sum(), metricName, serviceName, resourceAttrs, scopeName, scopeVersion, scopeAttrs, sumData)

				case pmetric.MetricTypeHistogram:
					e.collectHistogram(metric.Histogram(), metricName, serviceName, resourceAttrs, scopeName, scopeVersion, scopeAttrs, histogramData)

				case pmetric.MetricTypeSummary:
					e.collectSummary(metric.Summary(), metricName, serviceName, resourceAttrs, scopeName, scopeVersion, scopeAttrs, summaryData)

				case pmetric.MetricTypeExponentialHistogram:
					e.collectExpHistogram(metric.ExponentialHistogram(), metricName, serviceName, resourceAttrs, scopeName, scopeVersion, scopeAttrs, expHistData)
				}
			}
		}
	}

	totalRows := len(gaugeData.rows) + len(sumData.rows) + len(histogramData.rows) +
		len(summaryData.rows) + len(expHistData.rows)
	if totalRows == 0 {
		return nil
	}

	batches := []*metricsBatch{gaugeData, sumData, histogramData, summaryData, expHistData}
	for _, batch := range batches {
		if err := batch.insert(ctx, e.db); err != nil {
			return err
		}
	}

	e.logger.Debug("inserted metrics",
		zap.Int("datapoint_count", totalRows),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

func (e *metricsExporter) collectGauge(
	gauge pmetric.Gauge, metricName, serviceName string,
	resourceAttrs []byte, scopeName, scopeVersion string, scopeAttrs []byte,
	batch *metricsBatch,
) {
	dps := gauge.DataPoints()
	for i := range dps.Len() {
		dp := dps.At(i)
		attrs, _ := attributesToJSON(dp.Attributes())
		batch.addRow(
			dp.Timestamp().AsTime(),
			metricName,
			serviceName,
			getValue(dp),
			int32(dp.Flags()),
			resourceAttrs,
			scopeName,
			scopeVersion,
			scopeAttrs,
			attrs,
		)
	}
}

func (e *metricsExporter) collectSum(
	sum pmetric.Sum, metricName, serviceName string,
	resourceAttrs []byte, scopeName, scopeVersion string, scopeAttrs []byte,
	batch *metricsBatch,
) {
	dps := sum.DataPoints()
	for i := range dps.Len() {
		dp := dps.At(i)
		attrs, _ := attributesToJSON(dp.Attributes())
		batch.addRow(
			dp.Timestamp().AsTime(),
			dp.StartTimestamp().AsTime(),
			metricName,
			serviceName,
			getValue(dp),
			int32(dp.Flags()),
			sum.IsMonotonic(),
			aggregationTemporalityToString(sum.AggregationTemporality()),
			resourceAttrs,
			scopeName,
			scopeVersion,
			scopeAttrs,
			attrs,
		)
	}
}

func (e *metricsExporter) collectHistogram(
	hist pmetric.Histogram, metricName, serviceName string,
	resourceAttrs []byte, scopeName, scopeVersion string, scopeAttrs []byte,
	batch *metricsBatch,
) {
	dps := hist.DataPoints()
	for i := range dps.Len() {
		dp := dps.At(i)
		attrs, _ := attributesToJSON(dp.Attributes())
		batch.addRow(
			dp.Timestamp().AsTime(),
			dp.StartTimestamp().AsTime(),
			metricName,
			serviceName,
			int64(dp.Count()),
			dp.Sum(),
			dp.Min(),
			dp.Max(),
			int32(dp.Flags()),
			uint64SliceToString(dp.BucketCounts().AsRaw()),
			float64SliceToString(dp.ExplicitBounds().AsRaw()),
			aggregationTemporalityToString(hist.AggregationTemporality()),
			resourceAttrs,
			scopeName,
			scopeVersion,
			scopeAttrs,
			attrs,
		)
	}
}

func (e *metricsExporter) collectSummary(
	summary pmetric.Summary, metricName, serviceName string,
	resourceAttrs []byte, scopeName, scopeVersion string, scopeAttrs []byte,
	batch *metricsBatch,
) {
	dps := summary.DataPoints()
	for i := range dps.Len() {
		dp := dps.At(i)
		attrs, _ := attributesToJSON(dp.Attributes())
		qvs, qcs := extractQuantiles(dp.QuantileValues())
		batch.addRow(
			dp.Timestamp().AsTime(),
			dp.StartTimestamp().AsTime(),
			metricName,
			serviceName,
			int64(dp.Count()),
			dp.Sum(),
			int32(dp.Flags()),
			qvs,
			qcs,
			resourceAttrs,
			scopeName,
			scopeVersion,
			scopeAttrs,
			attrs,
		)
	}
}

func (e *metricsExporter) collectExpHistogram(
	expHist pmetric.ExponentialHistogram, metricName, serviceName string,
	resourceAttrs []byte, scopeName, scopeVersion string, scopeAttrs []byte,
	batch *metricsBatch,
) {
	dps := expHist.DataPoints()
	for i := range dps.Len() {
		dp := dps.At(i)
		attrs, _ := attributesToJSON(dp.Attributes())
		batch.addRow(
			dp.Timestamp().AsTime(),
			dp.StartTimestamp().AsTime(),
			metricName,
			serviceName,
			int64(dp.Count()),
			dp.Sum(),
			dp.Min(),
			dp.Max(),
			int32(dp.Scale()),
			int64(dp.ZeroCount()),
			int32(dp.Flags()),
			int32(dp.Positive().Offset()),
			uint64SliceToString(dp.Positive().BucketCounts().AsRaw()),
			int32(dp.Negative().Offset()),
			uint64SliceToString(dp.Negative().BucketCounts().AsRaw()),
			aggregationTemporalityToString(expHist.AggregationTemporality()),
			resourceAttrs,
			scopeName,
			scopeVersion,
			scopeAttrs,
			attrs,
		)
	}
}

// getValue extracts the numeric value from a NumberDataPoint.
func getValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return 0
	}
}

// aggregationTemporalityToString converts aggregation temporality to string.
func aggregationTemporalityToString(at pmetric.AggregationTemporality) string {
	switch at {
	case pmetric.AggregationTemporalityCumulative:
		return "Cumulative"
	case pmetric.AggregationTemporalityDelta:
		return "Delta"
	default:
		return "Unspecified"
	}
}

// uint64SliceToString converts a slice of uint64 to a comma-separated string.
func uint64SliceToString(slice []uint64) string {
	if len(slice) == 0 {
		return ""
	}
	strs := make([]string, len(slice))
	for i, v := range slice {
		strs[i] = strconv.FormatUint(v, 10)
	}
	return strings.Join(strs, ",")
}

// float64SliceToString converts a slice of float64 to a comma-separated string.
func float64SliceToString(slice []float64) string {
	if len(slice) == 0 {
		return ""
	}
	strs := make([]string, len(slice))
	for i, v := range slice {
		strs[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strings.Join(strs, ",")
}

// extractQuantiles extracts quantile markers and values from a SummaryDataPoint.
func extractQuantiles(qvs pmetric.SummaryDataPointValueAtQuantileSlice) (string, string) {
	if qvs.Len() == 0 {
		return "", ""
	}
	quantiles := make([]string, qvs.Len())
	values := make([]string, qvs.Len())
	for i := range qvs.Len() {
		qv := qvs.At(i)
		quantiles[i] = strconv.FormatFloat(qv.Quantile(), 'f', -1, 64)
		values[i] = strconv.FormatFloat(qv.Value(), 'f', -1, 64)
	}
	return strings.Join(quantiles, ","), strings.Join(values, ",")
}
