// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// language=ClickHouse SQL
	createExpHistogramTableSQL = `
CREATE TABLE IF NOT EXISTS %s_exponential_histogram (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Count Int64 CODEC(Delta, ZSTD(1)),
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
	INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_scope_attr_key mapKeys(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_scope_attr_value mapValues(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_attr_key mapKeys(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_attr_value mapValues(Attributes) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE MergeTree()
%s
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	// language=ClickHouse SQL
	insertExpHistogramTableSQL = `INSERT INTO %s_exponential_histogram (
	ResourceAttributes,
    ResourceSchemaUrl,
    ScopeName,
    ScopeVersion,
    ScopeAttributes,
    ScopeDroppedAttrCount,
    ScopeSchemaUrl,
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
	Max) VALUES `
	expHistogramValueCounts = 29
)

var expHistogramPlaceholders = newPlaceholder(expHistogramValueCounts)

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

	valueArgs := make([]any, e.count*expHistogramValueCounts)
	var b strings.Builder

	index := 0
	for _, model := range e.expHistogramModels {
		for i := 0; i < model.expHistogram.DataPoints().Len(); i++ {
			dp := model.expHistogram.DataPoints().At(i)
			b.WriteString(*expHistogramPlaceholders)

			valueArgs[index] = model.metadata.ResAttr
			valueArgs[index+1] = model.metadata.ResURL
			valueArgs[index+2] = model.metadata.ScopeInstr.Name()
			valueArgs[index+3] = model.metadata.ScopeInstr.Version()
			valueArgs[index+4] = attributesToMap(model.metadata.ScopeInstr.Attributes())
			valueArgs[index+5] = model.metadata.ScopeInstr.DroppedAttributesCount()
			valueArgs[index+6] = model.metadata.ScopeURL
			valueArgs[index+7] = model.metricName
			valueArgs[index+8] = model.metricDescription
			valueArgs[index+9] = model.metricUnit
			valueArgs[index+10] = attributesToMap(dp.Attributes())
			valueArgs[index+11] = dp.StartTimestamp().AsTime().UnixNano()
			valueArgs[index+12] = dp.Timestamp().AsTime().UnixNano()
			valueArgs[index+13] = dp.Count()
			valueArgs[index+14] = dp.Sum()
			valueArgs[index+15] = dp.Scale()
			valueArgs[index+16] = dp.ZeroCount()
			valueArgs[index+17] = dp.Positive().Offset()
			valueArgs[index+18] = convertSliceToArraySet(dp.Positive().BucketCounts().AsRaw())
			valueArgs[index+19] = dp.Negative().Offset()
			valueArgs[index+20] = convertSliceToArraySet(dp.Negative().BucketCounts().AsRaw())

			attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
			valueArgs[index+21] = attrs
			valueArgs[index+22] = times
			valueArgs[index+23] = values
			valueArgs[index+24] = traceIDs
			valueArgs[index+25] = spanIDs
			valueArgs[index+26] = uint32(dp.Flags())
			valueArgs[index+27] = dp.Min()
			valueArgs[index+28] = dp.Max()

			index += expHistogramValueCounts
		}
	}

	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf("%s %s", e.insertSQL, strings.TrimSuffix(b.String(), ",")), valueArgs...)
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

func (e *expHistogramMetrics) Add(resAttr map[string]string, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
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
