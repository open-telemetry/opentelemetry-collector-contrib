// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

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

func (h *histogramMetrics) insert(ctx context.Context, db driver.Conn) error {
	if h.count == 0 {
		return nil
	}

	processStart := time.Now()

	batch, err := db.PrepareBatch(ctx, h.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			logger.Warn("failed to close histogram metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range h.histogramModel {
		resAttr := AttributesToMap(model.metadata.ResAttr)
		scopeAttr := AttributesToMap(model.metadata.ScopeInstr.Attributes())
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.histogram.DataPoints().Len(); i++ {
			dp := model.histogram.DataPoints().At(i)
			attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
			err := batch.Append(
				resAttr,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttr,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				AttributesToMap(dp.Attributes()),
				dp.StartTimestamp().AsTime(),
				dp.Timestamp().AsTime(),
				dp.Count(),
				dp.Sum(),
				convertSliceToArraySet(dp.BucketCounts().AsRaw()),
				convertSliceToArraySet(dp.ExplicitBounds().AsRaw()),
				attrs,
				times,
				values,
				spanIDs,
				traceIDs,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.histogram.AggregationTemporality()),
			)
			if err != nil {
				return fmt.Errorf("failed to append histogram metric: %w", err)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if err := batch.Send(); err != nil {
		return fmt.Errorf("histogram metric insert failed: %w", err)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert histogram metrics",
		zap.Int("records", h.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (h *histogramMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	histogram, ok := metrics.(pmetric.Histogram)
	if !ok {
		return errors.New("metrics param is not type of Histogram")
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
