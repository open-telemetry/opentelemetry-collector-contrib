// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
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

	var batchValues []string

	for _, model := range e.expHistogramModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.expHistogram.DataPoints().Len(); i++ {
			dp := model.expHistogram.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())
			exemplarsJSON, _ := convertExemplarsJSON(dp.Exemplars())

			positiveBuckets, _ := json.Marshal(dp.Positive().BucketCounts().AsRaw())
			negativeBuckets, _ := json.Marshal(dp.Negative().BucketCounts().AsRaw())

			timestamp := dp.Timestamp().AsTime()
			startTimestamp := dp.StartTimestamp().AsTime()
			values := []interface{}{
				serviceName,
				model.metricName,
				time.Time(timestamp),
				resAttrJSON,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrJSON,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				model.metricDescription,
				model.metricUnit,
				attrsJSON,
				time.Time(startTimestamp),
				dp.Count(),
				dp.Sum(),
				int32(dp.Scale()),
				dp.ZeroCount(),
				int32(dp.Positive().Offset()),
				string(positiveBuckets),
				int32(dp.Negative().Offset()),
				string(negativeBuckets),
				exemplarsJSON,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.expHistogram.AggregationTemporality()),
			}

			batchValues = append(batchValues, internal.BuildValuesClause(values))
		}
	}

	if len(batchValues) > 0 {
		// Build complete INSERT statement with formatted values
		// Replace the VALUES (?, ?, ...) part with actual values
		valuesStart := strings.Index(e.insertSQL, "VALUES (")
		if valuesStart == -1 {
			return fmt.Errorf("failed to find VALUES clause in insert SQL")
		}
		insertSQL := e.insertSQL[:valuesStart] + "VALUES " + strings.Join(batchValues, ",")
		_, execErr := db.ExecContext(ctx, insertSQL)
		if execErr != nil {
			return fmt.Errorf("failed to execute exponential histogram metric insert: %w", execErr)
		}
	}

	logger.Debug("insert exponential histogram metrics", zap.Int("records", e.count))
	return nil
}

func (e *expHistogramMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	expHistogram := metrics.ExponentialHistogram()
	e.count += expHistogram.DataPoints().Len()
	e.expHistogramModels = append(e.expHistogramModels, &expHistogramModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		expHistogram: expHistogram,
	})
}
