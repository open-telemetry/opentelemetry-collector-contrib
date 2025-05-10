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
	createSummaryTableSQL = `
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
    ValueAtQuantiles Nested(
		Quantile Float64,
		Value Float64
	) CODEC(ZSTD(1)),
    Flags UInt32  CODEC(ZSTD(1)),
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
	insertSummaryTableSQL = `INSERT INTO %s (
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
    ValueAtQuantiles.Quantile,
	ValueAtQuantiles.Value,
    Flags) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
)

type summaryModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	summary           pmetric.Summary
}

type summaryMetrics struct {
	summaryModel []*summaryModel
	insertSQL    string
	count        int
}

func (s *summaryMetrics) insert(ctx context.Context, db *sql.DB) error {
	if s.count == 0 {
		return nil
	}
	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, s.insertSQL)
		if err != nil {
			return err
		}
		defer func() {
			_ = statement.Close()
		}()
		for _, model := range s.summaryModel {
			var serviceName string
			if v, ok := model.metadata.ResAttr[conventions.AttributeServiceName]; ok {
				serviceName = v
			}

			for i := 0; i < model.summary.DataPoints().Len(); i++ {
				dp := model.summary.DataPoints().At(i)
				quantiles, values := convertValueAtQuantile(dp.QuantileValues())

				_, err = statement.ExecContext(ctx,
					model.metadata.ResAttr,
					model.metadata.ResURL,
					model.metadata.ScopeInstr.Name(),
					model.metadata.ScopeInstr.Version(),
					attributesToMap(model.metadata.ScopeInstr.Attributes()),
					model.metadata.ScopeInstr.DroppedAttributesCount(),
					model.metadata.ScopeURL,
					serviceName,
					model.metricName,
					model.metricDescription,
					model.metricUnit,
					attributesToMap(dp.Attributes()),
					dp.StartTimestamp().AsTime(),
					dp.Timestamp().AsTime(),
					dp.Count(),
					dp.Sum(),
					quantiles,
					values,
					uint32(dp.Flags()),
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
		logger.Debug("insert summary metrics fail", zap.Duration("cost", duration))
		return fmt.Errorf("insert summary metrics fail:%w", err)
	}

	// TODO latency metrics
	logger.Debug("insert summary metrics", zap.Int("records", s.count),
		zap.Duration("cost", duration))
	return nil
}

func (s *summaryMetrics) Add(resAttr map[string]string, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	summary, ok := metrics.(pmetric.Summary)
	if !ok {
		return fmt.Errorf("metrics param is not type of Summary")
	}
	s.count += summary.DataPoints().Len()
	s.summaryModel = append(s.summaryModel, &summaryModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		summary: summary,
	})
	return nil
}
