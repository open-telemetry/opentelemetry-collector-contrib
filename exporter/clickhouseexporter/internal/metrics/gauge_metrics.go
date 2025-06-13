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
	count       int
}

func (g *gaugeMetrics) insert(ctx context.Context, db driver.Conn) error {
	if g.count == 0 {
		return nil
	}

	processStart := time.Now()

	batch, err := db.PrepareBatch(ctx, g.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			logger.Warn("failed to close gauge metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range g.gaugeModels {
		resAttr := AttributesToMap(model.metadata.ResAttr)
		scopeAttr := AttributesToMap(model.metadata.ScopeInstr.Attributes())
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.gauge.DataPoints().Len(); i++ {
			dp := model.gauge.DataPoints().At(i)
			attrs, times, values, traceIDs, spanIDs := convertExemplars(dp.Exemplars())
			appendErr := batch.Append(
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
				getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()),
				uint32(dp.Flags()),
				attrs,
				times,
				values,
				spanIDs,
				traceIDs,
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append gauge metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("gauge metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert gauge metrics",
		zap.Int("records", g.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (g *gaugeMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	gauge, ok := metrics.(pmetric.Gauge)
	if !ok {
		return errors.New("metrics param is not type of Gauge")
	}
	g.count += gauge.DataPoints().Len()
	g.gaugeModels = append(g.gaugeModels, &gaugeModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		gauge: gauge,
	})
	return nil
}
