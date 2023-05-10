// Copyright The OpenTelemetry Authors
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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// language=ClickHouse SQL
	createHistogramTableSQL = `
CREATE TABLE IF NOT EXISTS %s_histogram (
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
    BucketCounts Array(UInt64) CODEC(ZSTD(1)),
    ExplicitBounds Array(Float64) CODEC(ZSTD(1)),
	Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1)),
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
	insertHistogramTableSQL = `INSERT INTO %s_histogram (
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
	BucketCounts,
	ExplicitBounds,
  	Exemplars.FilteredAttributes,
	Exemplars.TimeUnix,
    Exemplars.Value,
    Exemplars.SpanId,
    Exemplars.TraceId,
	Flags,
	Min,
	Max) VALUES `
	histogramValueCounts = 25
)

var histogramPlaceholders = newPlaceholder(histogramValueCounts)

type histogramModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	histogram         pmetric.Histogram
}

type histogramMetrics struct {
	histogramModel []*histogramModel
	insertSQL      string
	count          int
}

func (h *histogramMetrics) insert(ctx context.Context, db *sql.DB) error {
	if h.count == 0 {
		return nil
	}

	valueArgs := make([]any, h.count*histogramValueCounts)
	var b strings.Builder

	index := 0
	for _, model := range h.histogramModel {
		for i := 0; i < model.histogram.DataPoints().Len(); i++ {
			dp := model.histogram.DataPoints().At(i)
			b.WriteString(*histogramPlaceholders)

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
			valueArgs[index+15] = convertSliceToArraySet(dp.BucketCounts().AsRaw())
			valueArgs[index+16] = convertSliceToArraySet(dp.ExplicitBounds().AsRaw())

			attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
			valueArgs[index+17] = attrs
			valueArgs[index+18] = times
			valueArgs[index+19] = values
			valueArgs[index+20] = traceIDs
			valueArgs[index+21] = spanIDs
			valueArgs[index+22] = uint32(dp.Flags())
			valueArgs[index+23] = dp.Min()
			valueArgs[index+24] = dp.Max()

			index += histogramValueCounts
		}
	}

	start := time.Now()
	err := doWithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf("%s %s", h.insertSQL, strings.TrimSuffix(b.String(), ",")), valueArgs...)
		return err
	})
	duration := time.Since(start)
	if err != nil {
		logger.Debug("insert histogram metrics fail", zap.Duration("cost", duration))
		return fmt.Errorf("insert histogram metrics fail:%w", err)
	}

	// TODO latency metrics
	logger.Debug("insert histogram metrics", zap.Int("records", h.count),
		zap.Duration("cost", duration))
	return nil
}

func (h *histogramMetrics) Add(resAttr map[string]string, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	histogram, ok := metrics.(pmetric.Histogram)
	if !ok {
		return fmt.Errorf("metrics param is not type of Histogram")
	}
	h.count += histogram.DataPoints().Len()
	h.histogramModel = append(h.histogramModel, &histogramModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		histogram: histogram,
	})
	return nil
}
