// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter"

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal/sqltemplates"
)

const (
	gaugeColumnCount     = 17
	sumColumnCount       = 19
	histogramColumnCount = 23
	summaryColumnCount   = 18
)

type metricsExporter struct {
	db *sql.DB

	logger *zap.Logger
	cfg    *Config

	gaugeInsertPrefix     string
	gaugeValuePlaceholder string
	sumInsertPrefix       string
	sumValuePlaceholder   string
	histInsertPrefix      string
	histValuePlaceholder  string
	summInsertPrefix      string
	summValuePlaceholder  string
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) *metricsExporter {
	return &metricsExporter{
		logger:                logger,
		cfg:                   cfg,
		gaugeInsertPrefix:     internal.BuildInsertPrefix(sqltemplates.MetricsGaugeInsert, cfg.Database, cfg.gaugeTableName()),
		gaugeValuePlaceholder: internal.BuildValuePlaceholder(gaugeColumnCount),
		sumInsertPrefix:       internal.BuildInsertPrefix(sqltemplates.MetricsSumInsert, cfg.Database, cfg.sumTableName()),
		sumValuePlaceholder:   internal.BuildValuePlaceholder(sumColumnCount),
		histInsertPrefix:      internal.BuildInsertPrefix(sqltemplates.MetricsHistogramInsert, cfg.Database, cfg.histogramTableName()),
		histValuePlaceholder:  internal.BuildValuePlaceholder(histogramColumnCount),
		summInsertPrefix:      internal.BuildInsertPrefix(sqltemplates.MetricsSummaryInsert, cfg.Database, cfg.summaryTableName()),
		summValuePlaceholder:  internal.BuildValuePlaceholder(summaryColumnCount),
	}
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	dsn := e.cfg.buildDSN()

	if e.cfg.CreateSchema {
		initDB, err := internal.NewMySQLClientNoDB(dsn)
		if err != nil {
			return err
		}
		if err := internal.CreateDatabase(ctx, initDB, e.cfg.Database); err != nil {
			_ = initDB.Close()
			return err
		}
		_ = initDB.Close()
	}

	db, err := internal.NewMySQLClient(dsn)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		tables := map[string]string{
			e.cfg.gaugeTableName():     sqltemplates.MetricsGaugeCreateTable,
			e.cfg.sumTableName():       sqltemplates.MetricsSumCreateTable,
			e.cfg.histogramTableName(): sqltemplates.MetricsHistogramCreateTable,
			e.cfg.summaryTableName():   sqltemplates.MetricsSummaryCreateTable,
		}
		for tableName, tmpl := range tables {
			ddl := renderCreateTableSQL(tmpl, e.cfg.Database, tableName)
			if err := internal.CreateTable(ctx, e.db, ddl); err != nil {
				return fmt.Errorf("create metrics table %s: %w", tableName, err)
			}
		}
	}

	if e.cfg.TTL > 0 {
		metricsTables := []string{
			e.cfg.gaugeTableName(),
			e.cfg.sumTableName(),
			e.cfg.histogramTableName(),
			e.cfg.summaryTableName(),
		}
		for _, tableName := range metricsTables {
			if err := internal.CreateTTLEvent(ctx, e.db, e.cfg.Database, tableName, "time_unix", e.cfg.TTL); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	start := time.Now()

	var gaugeRows, sumRows, histRows, summRows [][]any

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resAttr := rm.Resource().Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON := internal.AttributesToJSON(resAttr)
		resURL := rm.SchemaUrl()

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scope := sm.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeAttrJSON := internal.AttributesToJSON(scope.Attributes())
			scopeDroppedAttrCount := scope.DroppedAttributesCount()
			scopeURL := sm.SchemaUrl()

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricName := m.Name()
				metricDesc := m.Description()
				metricUnit := m.Unit()

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					gaugeRows = collectGaugeRows(gaugeRows, m.Gauge(),
						resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
						serviceName, metricName, metricDesc, metricUnit)

				case pmetric.MetricTypeSum:
					sumRows = collectSumRows(sumRows, m.Sum(),
						resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
						serviceName, metricName, metricDesc, metricUnit)

				case pmetric.MetricTypeHistogram:
					histRows = collectHistogramRows(histRows, m.Histogram(),
						resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
						serviceName, metricName, metricDesc, metricUnit)

				case pmetric.MetricTypeSummary:
					summRows = collectSummaryRows(summRows, m.Summary(),
						resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
						serviceName, metricName, metricDesc, metricUnit)
				}
			}
		}
	}

	metricCount := len(gaugeRows) + len(sumRows) + len(histRows) + len(summRows)

	if len(gaugeRows) > 0 {
		if err := internal.BatchInsert(ctx, e.db, e.gaugeInsertPrefix, e.gaugeValuePlaceholder, gaugeRows, internal.DefaultBatchSize); err != nil {
			return fmt.Errorf("batch insert gauge: %w", err)
		}
	}
	if len(sumRows) > 0 {
		if err := internal.BatchInsert(ctx, e.db, e.sumInsertPrefix, e.sumValuePlaceholder, sumRows, internal.DefaultBatchSize); err != nil {
			return fmt.Errorf("batch insert sum: %w", err)
		}
	}
	if len(histRows) > 0 {
		if err := internal.BatchInsert(ctx, e.db, e.histInsertPrefix, e.histValuePlaceholder, histRows, internal.DefaultBatchSize); err != nil {
			return fmt.Errorf("batch insert histogram: %w", err)
		}
	}
	if len(summRows) > 0 {
		if err := internal.BatchInsert(ctx, e.db, e.summInsertPrefix, e.summValuePlaceholder, summRows, internal.DefaultBatchSize); err != nil {
			return fmt.Errorf("batch insert summary: %w", err)
		}
	}

	duration := time.Since(start)
	e.logger.Debug("insert metrics",
		zap.Int("records", metricCount),
		zap.String("cost", duration.String()))

	return nil
}

func collectGaugeRows(rows [][]any, gauge pmetric.Gauge,
	resAttrJSON []byte, resURL, scopeName, scopeVersion string, scopeAttrJSON []byte, scopeDroppedAttrCount uint32, scopeURL,
	serviceName, metricName, metricDesc, metricUnit string,
) [][]any {
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		rows = append(rows, []any{
			resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
			serviceName, metricName, metricDesc, metricUnit,
			internal.AttributesToJSON(dp.Attributes()), dp.StartTimestamp().AsTime(), dp.Timestamp().AsTime(),
			getNumberValue(dp), dp.Flags(), convertExemplarsToJSON(dp.Exemplars()),
		})
	}
	return rows
}

func collectSumRows(rows [][]any, sum pmetric.Sum,
	resAttrJSON []byte, resURL, scopeName, scopeVersion string, scopeAttrJSON []byte, scopeDroppedAttrCount uint32, scopeURL,
	serviceName, metricName, metricDesc, metricUnit string,
) [][]any {
	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		rows = append(rows, []any{
			resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
			serviceName, metricName, metricDesc, metricUnit,
			internal.AttributesToJSON(dp.Attributes()), dp.StartTimestamp().AsTime(), dp.Timestamp().AsTime(),
			getNumberValue(dp), dp.Flags(),
			int32(sum.AggregationTemporality()), sum.IsMonotonic(),
			convertExemplarsToJSON(dp.Exemplars()),
		})
	}
	return rows
}

func collectHistogramRows(rows [][]any, histogram pmetric.Histogram,
	resAttrJSON []byte, resURL, scopeName, scopeVersion string, scopeAttrJSON []byte, scopeDroppedAttrCount uint32, scopeURL,
	serviceName, metricName, metricDesc, metricUnit string,
) [][]any {
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)
		bucketCountsJSON, _ := json.Marshal(dp.BucketCounts().AsRaw())
		explicitBoundsJSON, _ := json.Marshal(dp.ExplicitBounds().AsRaw())

		var sumVal *float64
		if dp.HasSum() {
			s := dp.Sum()
			sumVal = &s
		}
		var minVal *float64
		if dp.HasMin() {
			m := dp.Min()
			minVal = &m
		}
		var maxVal *float64
		if dp.HasMax() {
			m := dp.Max()
			maxVal = &m
		}

		rows = append(rows, []any{
			resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
			serviceName, metricName, metricDesc, metricUnit,
			internal.AttributesToJSON(dp.Attributes()), dp.StartTimestamp().AsTime(), dp.Timestamp().AsTime(),
			dp.Count(), sumVal,
			bucketCountsJSON, explicitBoundsJSON,
			dp.Flags(), minVal, maxVal,
			int32(histogram.AggregationTemporality()),
			convertExemplarsToJSON(dp.Exemplars()),
		})
	}
	return rows
}

func collectSummaryRows(rows [][]any, summary pmetric.Summary,
	resAttrJSON []byte, resURL, scopeName, scopeVersion string, scopeAttrJSON []byte, scopeDroppedAttrCount uint32, scopeURL,
	serviceName, metricName, metricDesc, metricUnit string,
) [][]any {
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)
		rows = append(rows, []any{
			resAttrJSON, resURL, scopeName, scopeVersion, scopeAttrJSON, scopeDroppedAttrCount, scopeURL,
			serviceName, metricName, metricDesc, metricUnit,
			internal.AttributesToJSON(dp.Attributes()), dp.StartTimestamp().AsTime(), dp.Timestamp().AsTime(),
			dp.Count(), dp.Sum(),
			convertQuantileValuesToJSON(dp.QuantileValues()),
			dp.Flags(),
		})
	}
	return rows
}

// numberDataPoint is a generic interface for number data points.
type numberDataPoint interface {
	IntValue() int64
	DoubleValue() float64
	ValueType() pmetric.NumberDataPointValueType
}

func getNumberValue(dp numberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	default:
		return 0.0
	}
}

type exemplarJSON struct {
	FilteredAttributes map[string]string `json:"filtered_attributes,omitempty"`
	Timestamp          time.Time         `json:"timestamp"`
	Value              float64           `json:"value"`
	SpanID             string            `json:"span_id"`
	TraceID            string            `json:"trace_id"`
}

func convertExemplarsToJSON(exemplars pmetric.ExemplarSlice) []byte {
	n := exemplars.Len()
	if n == 0 {
		return nil
	}
	result := make([]exemplarJSON, 0, n)
	for i := 0; i < n; i++ {
		ex := exemplars.At(i)
		attrs := make(map[string]string)
		ex.FilteredAttributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})

		var value float64
		switch ex.ValueType() {
		case pmetric.ExemplarValueTypeDouble:
			value = ex.DoubleValue()
		case pmetric.ExemplarValueTypeInt:
			value = float64(ex.IntValue())
		}

		traceID := ex.TraceID()
		spanID := ex.SpanID()
		result = append(result, exemplarJSON{
			FilteredAttributes: attrs,
			Timestamp:          ex.Timestamp().AsTime(),
			Value:              value,
			TraceID:            hex.EncodeToString(traceID[:]),
			SpanID:             hex.EncodeToString(spanID[:]),
		})
	}
	b, _ := json.Marshal(result)
	return b
}

type quantileValueJSON struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

func convertQuantileValuesToJSON(quantiles pmetric.SummaryDataPointValueAtQuantileSlice) []byte {
	n := quantiles.Len()
	if n == 0 {
		return nil
	}
	result := make([]quantileValueJSON, 0, n)
	for i := 0; i < n; i++ {
		q := quantiles.At(i)
		result = append(result, quantileValueJSON{
			Quantile: q.Quantile(),
			Value:    q.Value(),
		})
	}
	b, _ := json.Marshal(result)
	return b
}
