// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

const (
	// language=ClickHouse SQL
	createExpHistogramTableSQL = `
CREATE TABLE IF NOT EXISTS %s %s (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Count UInt64 CODEC(Delta, ZSTD(1)),
    Sum Float64 CODEC(ZSTD(1)),
    Scale Int32 CODEC(ZSTD(1)),
    ZeroCount UInt64 CODEC(ZSTD(1)),
	PositiveOffset Int32 CODEC(ZSTD(1)),
	PositiveBucketCounts Array(UInt64) CODEC(ZSTD(1)),
	NegativeOffset Int32 CODEC(ZSTD(1)),
	NegativeBucketCounts Array(UInt64) CODEC(ZSTD(1)),
	Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    Flags UInt32  CODEC(ZSTD(1)),
    Min Float64 CODEC(ZSTD(1)),
    Max Float64 CODEC(ZSTD(1)),
		AggregationTemporality Int32 CODEC(ZSTD(1)),
	INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_scope_attr_key mapKeys(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_scope_attr_value mapValues(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_attr_key mapKeys(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_attr_value mapValues(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
%s
PARTITION BY toDate(TimeUnix)
ORDER BY (ServiceName, MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	// language=ClickHouse SQL
	insertExpHistogramTableSQL = `INSERT INTO %s (
	ResourceAttributes,
    ResourceSchemaUrl,
    ScopeName,
    ScopeVersion,
    ScopeAttributes,
    ScopeDroppedAttrCount,
    ScopeSchemaUrl,
    ServiceName,
    MetricName,
    MetricDescription,
    MetricUnit,
    Attributes,
	StartTimeUnix,
	TimeUnix,
	Count,
	Sum,
    Scale,
    ZeroCount,
	PositiveOffset,
	PositiveBucketCounts,
	NegativeOffset,
	NegativeBucketCounts,
  	Exemplars.FilteredAttributes,
	Exemplars.TimeUnix,
    Exemplars.Value,
    Exemplars.SpanId,
    Exemplars.TraceId,
	Flags,
	Min,
	Max,
	AggregationTemporality) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
)

type expHistogramModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	expHistogram      pmetric.ExponentialHistogram
}

type expHistogramMetrics struct {
	expHistogramModels []*expHistogramModel
	insertSQL          string
	count              int
}

func (e *expHistogramMetrics) insert(ctx context.Context, db *sql.DB) error {
	if e.count == 0 {
		return nil
	}

	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertSQL)
		if err != nil {
			return err
		}

		defer func() {
			_ = statement.Close()
		}()

		for _, model := range e.expHistogramModels {
			serviceName, _ := model.metadata.ResAttr.Get(conventions.AttributeServiceName)

			for i := 0; i < model.expHistogram.DataPoints().Len(); i++ {
				dp := model.expHistogram.DataPoints().At(i)
				attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
				_, err = statement.ExecContext(ctx,
					AttributesToMap(model.metadata.ResAttr),
					model.metadata.ResURL,
					model.metadata.ScopeInstr.Name(),
					model.metadata.ScopeInstr.Version(),
					AttributesToMap(model.metadata.ScopeInstr.Attributes()),
					model.metadata.ScopeInstr.DroppedAttributesCount(),
					model.metadata.ScopeURL,
					serviceName.AsString(),
					model.metricName,
					model.metricDescription,
					model.metricUnit,
					AttributesToMap(dp.Attributes()),
					dp.StartTimestamp().AsTime(),
					dp.Timestamp().AsTime(),
					dp.Count(),
					dp.Sum(),
					dp.Scale(),
					dp.ZeroCount(),
					dp.Positive().Offset(),
					convertSliceToArraySet(dp.Positive().BucketCounts().AsRaw()),
					dp.Negative().Offset(),
					convertSliceToArraySet(dp.Negative().BucketCounts().AsRaw()),
					attrs,
					times,
					values,
					spanIDs,
					traceIDs,
					uint32(dp.Flags()),
					dp.Min(),
					dp.Max(),
					int32(model.expHistogram.AggregationTemporality()),
				)
				if err != nil {
					return fmt.Errorf("ExecContext:%w", err)
				}
			}
		}
		return err
	})
	duration := time.Since(start)
	if err != nil {
		logger.Debug("insert exponential histogram metrics fail", zap.Duration("cost", duration))
		return fmt.Errorf("insert exponential histogram metrics fail:%w", err)
	}

	// TODO latency metrics
	logger.Debug("insert exponential histogram metrics", zap.Int("records", e.count),
		zap.Duration("cost", duration))
	return nil
}

func (e *expHistogramMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	expHistogram, ok := metrics.(pmetric.ExponentialHistogram)
	if !ok {
		return fmt.Errorf("metrics param is not type of ExponentialHistogram")
	}
	e.count += expHistogram.DataPoints().Len()
	e.expHistogramModels = append(e.expHistogramModels, &expHistogramModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		expHistogram: expHistogram,
	})

	return nil
}
