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

type sumModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	sum               pmetric.Sum
}

type sumMetrics struct {
	sumModel  []*sumModel
	insertSQL string
	count     int
}

func (s *sumMetrics) insert(ctx context.Context, db driver.Conn) error {
	if s.count == 0 {
		return nil
	}

	processStart := time.Now()

	batch, err := db.PrepareBatch(ctx, s.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			logger.Warn("failed to close sum metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range s.sumModel {
		resAttr := AttributesToMap(model.metadata.ResAttr)
		scopeAttr := AttributesToMap(model.metadata.ScopeInstr.Attributes())
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.sum.DataPoints().Len(); i++ {
			dp := model.sum.DataPoints().At(i)
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
				int32(model.sum.AggregationTemporality()),
				model.sum.IsMonotonic(),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append sum metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("sum metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert sum metrics",
		zap.Int("records", s.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (s *sumMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	sum, ok := metrics.(pmetric.Sum)
	if !ok {
		return errors.New("metrics param is not type of Sum")
	}
	s.count += sum.DataPoints().Len()
	s.sumModel = append(s.sumModel, &sumModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		sum: sum,
	})
	return nil
}
