// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
)

type histogramModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	histogram         pmetric.Histogram
}

type histogramMetrics struct {
	histogramModels []*histogramModel
	insertSQL       string
	count           int
}

func (h *histogramMetrics) insert(ctx context.Context, db *sql.DB) error {
	if h.count == 0 {
		return nil
	}

	stmt, err := db.PrepareContext(ctx, h.insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare histogram insert statement: %w", err)
	}
	defer stmt.Close()

	for _, model := range h.histogramModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.histogram.DataPoints().Len(); i++ {
			dp := model.histogram.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())
			exemplarsJSON, _ := convertExemplarsJSON(dp.Exemplars())

			bucketCounts, _ := json.Marshal(dp.BucketCounts().AsRaw())
			explicitBounds, _ := json.Marshal(dp.ExplicitBounds().AsRaw())

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
				dp.Count(),
				dp.Sum(),
				string(bucketCounts),
				string(explicitBounds),
				exemplarsJSON,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.histogram.AggregationTemporality()),
			)
			if execErr != nil {
				return fmt.Errorf("failed to execute histogram metric insert: %w", execErr)
			}
		}
	}

	logger.Debug("insert histogram metrics", zap.Int("records", h.count))
	return nil
}

func (h *histogramMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	histogram := metrics.Histogram()
	h.count += histogram.DataPoints().Len()
	h.histogramModels = append(h.histogramModels, &histogramModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		histogram: histogram,
	})
}
