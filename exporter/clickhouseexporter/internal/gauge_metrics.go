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
	createGaugeTableSQL = `
CREATE TABLE IF NOT EXISTS %s_gauge (
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
    Flags UInt32 CODEC(ZSTD(1)),
    Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1))
) ENGINE MergeTree()
%s
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	// language=ClickHouse SQL
	insertGaugeTableSQL = `INSERT INTO %s_gauge (
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
    Exemplars.TraceId) VALUES `
	gaugePlaceholders = "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
)

type gaugeModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	gauge             pmetric.Gauge
}

type gaugeMetrics struct {
	gaugeModels []*gaugeModel
	insertSQL   string
}

func (g *gaugeMetrics) insert(ctx context.Context, db *sql.DB, logger *zap.Logger) error {
	if len(g.gaugeModels) == 0 {
		return nil
	}

	var valuePlaceholders []string
	var valueArgs []interface{}

	for _, model := range g.gaugeModels {
		for i := 0; i < model.gauge.DataPoints().Len(); i++ {
			dp := model.gauge.DataPoints().At(i)
			valuePlaceholders = append(valuePlaceholders, gaugePlaceholders)

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
		}
	}

	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf("%s %s", g.insertSQL, strings.Join(valuePlaceholders, ",")), valueArgs...)
		return err
	})
	duration := time.Since(start)
	if err != nil {
		logger.Debug("insert gauge metrics fail", zap.Duration("cost", duration))
		return fmt.Errorf("insert gauge metrics fail:%w", err)
	}

	// TODO latency metrics
	logger.Debug("insert gauge metrics", zap.Int("records", len(valuePlaceholders)),
		zap.Duration("cost", duration))
	return nil
}

func (g *gaugeMetrics) Add(metrics any, metaData *MetricsMetaData, name string, description string, unit string) error {
	gauge, ok := metrics.(pmetric.Gauge)
	if !ok {
		return fmt.Errorf("metrics param is not type of Gauge")
	}
	g.gaugeModels = append(g.gaugeModels, &gaugeModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata:          metaData,
		gauge:             gauge,
	})
	return nil
}
