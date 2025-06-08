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

func (e *expHistogramMetrics) insert(ctx context.Context, db driver.Conn) error {
	if e.count == 0 {
		return nil
	}

	processStart := time.Now()

	batch, err := db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			logger.Warn("failed to close exponential histogram metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range e.expHistogramModels {
		resAttr := AttributesToMap(model.metadata.ResAttr)
		scopeAttr := AttributesToMap(model.metadata.ScopeInstr.Attributes())
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.expHistogram.DataPoints().Len(); i++ {
			dp := model.expHistogram.DataPoints().At(i)
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
				dp.Count(),
				dp.Sum(),
				dp.Scale(),
				dp.ZeroCount(),
				dp.Positive().Offset(),
				convertSliceToArraySet(dp.Positive().BucketCounts().AsRaw()),
				dp.Negative().Offset(),
				convertSliceToArraySet(dp.Negative().BucketCounts().AsRaw()),
				attrs,
				times,
				values,
				spanIDs,
				traceIDs,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.expHistogram.AggregationTemporality()),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append exponential histogram metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("exponential histogram metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert exponential histogram metrics",
		zap.Int("records", e.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (e *expHistogramMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics any, name string, description string, unit string) error {
	expHistogram, ok := metrics.(pmetric.ExponentialHistogram)
	if !ok {
		return errors.New("metrics param is not type of ExponentialHistogram")
	}
	e.count += expHistogram.DataPoints().Len()
	e.expHistogramModels = append(e.expHistogramModels, &expHistogramModel{
		metricName:        name,
		metricDescription: description,
		metricUnit:        unit,
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		expHistogram: expHistogram,
	})

	return nil
}
