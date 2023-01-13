// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// language=ClickHouse SQL
	createSumTableSQL = `
CREATE TABLE IF NOT EXISTS %s_sum (
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
	Value Float64 CODEC(ZSTD(1)),
	Flags UInt32  CODEC(ZSTD(1)),
    Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    AggTemp Int32 CODEC(ZSTD(1)),
	IsMonotonic Boolean CODEC(Delta, ZSTD(1))
) ENGINE MergeTree()
%s
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	// language=ClickHouse SQL
	insertSumTableSQL = `INSERT INTO %s_sum (
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
    TimeUnix,
    Value,
    Flags,
    Exemplars.FilteredAttributes,
	Exemplars.TimeUnix,
    Exemplars.Value,
    Exemplars.SpanId,
    Exemplars.TraceId,
	AggTemp,
	IsMonotonic) VALUES `
	sumPlaceholders = "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
)

type sumModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	sum               pmetric.Sum
}

type sumMetrics struct {
	sumModel  []*sumModel
	insertSQL string
}

func (s *sumMetrics) insert(ctx context.Context, db *sql.DB, logger *zap.Logger) error {
	if len(s.sumModel) == 0 {
		return nil
	}

	var valuePlaceholders []string
	var valueArgs []interface{}

	for _, model := range s.sumModel {
		for i := 0; i < model.sum.DataPoints().Len(); i++ {
			dp := model.sum.DataPoints().At(i)
			valuePlaceholders = append(valuePlaceholders, sumPlaceholders)

			valueArgs = append(valueArgs, model.metadata.ResAttr)
			valueArgs = append(valueArgs, model.metadata.ResURL)
			valueArgs = append(valueArgs, model.metadata.ScopeInstr.Name())
			valueArgs = append(valueArgs, model.metadata.ScopeInstr.Version())
			valueArgs = append(valueArgs, attributesToMap(model.metadata.ScopeInstr.Attributes()))
			valueArgs = append(valueArgs, model.metadata.ScopeInstr.DroppedAttributesCount())
			valueArgs = append(valueArgs, model.metadata.ScopeURL)
			valueArgs = append(valueArgs, model.metricName)
			valueArgs = append(valueArgs, model.metricDescription)
			valueArgs = append(valueArgs, model.metricUnit)
			valueArgs = append(valueArgs, attributesToMap(dp.Attributes()))
			valueArgs = append(valueArgs, dp.Timestamp().AsTime().UnixNano())
			valueArgs = append(valueArgs, getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()))
			valueArgs = append(valueArgs, uint32(dp.Flags()))

			attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
			valueArgs = append(valueArgs, attrs)
			valueArgs = append(valueArgs, times)
			valueArgs = append(valueArgs, values)
			valueArgs = append(valueArgs, traceIDs)
			valueArgs = append(valueArgs, spanIDs)
			valueArgs = append(valueArgs, int32(model.sum.AggregationTemporality()))
			valueArgs = append(valueArgs, model.sum.IsMonotonic())
		}
	}

	if len(valuePlaceholders) == 0 {
		return nil
	}

	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf("%s %s", s.insertSQL, strings.Join(valuePlaceholders, ",")), valueArgs...)
		return err
	})
	duration := time.Since(start)
	if err != nil {
		logger.Debug("insert sum metrics fail", zap.Duration("cost", duration))
		return fmt.Errorf("insert sum metrics fail:%w", err)
	}

	// TODO latency metrics
	logger.Debug("insert sum metrics", zap.Int("records", len(valuePlaceholders)),
		zap.Duration("cost", duration))
	return nil
}

func (s *sumMetrics) Add(metrics any, metaData *MetricsMetaData, name string, description string, unit string) error {
	sum, ok := metrics.(pmetric.Sum)
	if !ok {
		return fmt.Errorf("metrics param is not type of Sum")
	}
	s.sumModel = append(s.sumModel, &sumModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata:          metaData,
		sum:               sum,
	})
	return nil
}
