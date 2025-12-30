// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
)

type sumModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	sum               pmetric.Sum
}

type sumMetrics struct {
	sumModels []*sumModel
	insertSQL string
	count     int
}

func (s *sumMetrics) insert(ctx context.Context, db *sql.DB) error {
	if s.count == 0 {
		return nil
	}

	var batchValues []string

	for _, model := range s.sumModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.sum.DataPoints().Len(); i++ {
			dp := model.sum.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())
			exemplarsJSON, _ := convertExemplarsJSON(dp.Exemplars())

			values := []interface{}{
				serviceName,
				model.metricName,
				dp.Timestamp().AsTime(),
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
				dp.StartTimestamp().AsTime(),
				getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()),
				uint32(dp.Flags()),
				exemplarsJSON,
				int32(model.sum.AggregationTemporality()),
				model.sum.IsMonotonic(),
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
			return fmt.Errorf("failed to execute sum metric insert: %w", execErr)
		}
	}

	logger.Debug("insert sum metrics", zap.Int("records", s.count))
	return nil
}

func (s *sumMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	sum := metrics.Sum()
	s.count += sum.DataPoints().Len()
	s.sumModels = append(s.sumModels, &sumModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		sum: sum,
	})
}
