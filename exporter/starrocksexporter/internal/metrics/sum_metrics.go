// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"database/sql"
	"fmt"

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

	stmt, err := db.PrepareContext(ctx, s.insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare sum insert statement: %w", err)
	}
	defer stmt.Close()

	for _, model := range s.sumModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.sum.DataPoints().Len(); i++ {
			dp := model.sum.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())
			exemplarsJSON, _ := convertExemplarsJSON(dp.Exemplars())

			_, execErr := stmt.ExecContext(ctx,
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
			)
			if execErr != nil {
				return fmt.Errorf("failed to execute sum metric insert: %w", execErr)
			}
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
