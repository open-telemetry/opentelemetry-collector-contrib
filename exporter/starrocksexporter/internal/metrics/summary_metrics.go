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

type summaryModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	summary           pmetric.Summary
}

type summaryMetrics struct {
	summaryModels []*summaryModel
	insertSQL     string
	count         int
}

func (s *summaryMetrics) insert(ctx context.Context, db *sql.DB) error {
	if s.count == 0 {
		return nil
	}

	var batchValues []string

	for _, model := range s.summaryModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.summary.DataPoints().Len(); i++ {
			dp := model.summary.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())

			quantiles := make([]map[string]interface{}, 0, dp.QuantileValues().Len())
			for j := 0; j < dp.QuantileValues().Len(); j++ {
				qv := dp.QuantileValues().At(j)
				quantiles = append(quantiles, map[string]interface{}{
					"quantile": qv.Quantile(),
					"value":    qv.Value(),
				})
			}
			quantilesJSON, _ := json.Marshal(quantiles)

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
				string(quantilesJSON),
				uint32(dp.Flags()),
			}

			batchValues = append(batchValues, internal.BuildValuesClause(values))
		}
	}

	if len(batchValues) > 0 {
		// Build complete INSERT statement with formatted values
		// Replace the VALUES (?, ?, ...) part with actual values
		valuesStart := strings.Index(s.insertSQL, "VALUES (")
		if valuesStart == -1 {
			return fmt.Errorf("failed to find VALUES clause in insert SQL")
		}
		insertSQL := s.insertSQL[:valuesStart] + "VALUES " + strings.Join(batchValues, ",")
		_, execErr := db.ExecContext(ctx, insertSQL)
		if execErr != nil {
			return fmt.Errorf("failed to execute summary metric insert: %w", execErr)
		}
	}

	logger.Debug("insert summary metrics", zap.Int("records", s.count))
	return nil
}

func (s *summaryMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	summary := metrics.Summary()
	s.count += summary.DataPoints().Len()
	s.summaryModels = append(s.summaryModels, &summaryModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		summary: summary,
	})
}
