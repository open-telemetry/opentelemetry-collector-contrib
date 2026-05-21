// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

// convertExemplarsJSON converts exemplars to slices suitable for JSON-typed columns.
// FilteredAttributes are serialized as JSON strings rather than ordered maps.
func convertExemplarsJSON(exemplars pmetric.ExemplarSlice) (clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet, clickhouse.ArraySet, error) {
	n := exemplars.Len()
	if n == 0 {
		return nil, nil, nil, nil, nil, nil
	}
	attrs := make(clickhouse.ArraySet, 0, n)
	times := make(clickhouse.ArraySet, 0, n)
	values := make(clickhouse.ArraySet, 0, n)
	traceIDs := make(clickhouse.ArraySet, 0, n)
	spanIDs := make(clickhouse.ArraySet, 0, n)
	for i := range n {
		exemplar := exemplars.At(i)
		attrBytes, err := json.Marshal(exemplar.FilteredAttributes().AsRaw())
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("failed to marshal exemplar filtered attributes: %w", err)
		}
		attrs = append(attrs, string(attrBytes))
		times = append(times, exemplar.Timestamp().AsTime())
		values = append(values, getValue(exemplar.IntValue(), exemplar.DoubleValue(), exemplar.ValueType()))
		traceID, spanID := exemplar.TraceID(), exemplar.SpanID()
		traceIDs = append(traceIDs, traceID.String())
		spanIDs = append(spanIDs, spanID.String())
	}
	return attrs, times, values, traceIDs, spanIDs, nil
}

// marshalAttrs serializes a pcommon.Map to JSON bytes and extracts flattened attribute keys.
func marshalAttrs(m pcommon.Map) ([]byte, []string, error) {
	b, err := json.Marshal(m.AsRaw())
	if err != nil {
		return nil, nil, err
	}
	return b, internal.UniqueFlattenedAttributes(m), nil
}

// --- gaugeMetricsJSON ---

type gaugeMetricsJSON struct {
	gaugeModels []*gaugeModel
	insertSQL   string
	count       int
}

func (g *gaugeMetricsJSON) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	gauge := metrics.Gauge()
	g.count += gauge.DataPoints().Len()
	g.gaugeModels = append(g.gaugeModels, &gaugeModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		gauge: gauge,
	})
}

func (g *gaugeMetricsJSON) insert(ctx context.Context, db driver.Conn) error {
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
			logger.Warn("failed to close gauge json metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range g.gaugeModels {
		resAttrBytes, resAttrKeys, err := marshalAttrs(model.metadata.ResAttr)
		if err != nil {
			return fmt.Errorf("failed to marshal gauge resource attributes: %w", err)
		}
		scopeAttrBytes, scopeAttrKeys, err := marshalAttrs(model.metadata.ScopeInstr.Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal gauge scope attributes: %w", err)
		}
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.gauge.DataPoints().Len(); i++ {
			dp := model.gauge.DataPoints().At(i)
			attrBytes, attrKeys, err := marshalAttrs(dp.Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal gauge data point attributes: %w", err)
			}
			exemplarAttrs, exemplarTimes, exemplarValues, exemplarTraceIDs, exemplarSpanIDs, exemplarErr := convertExemplarsJSON(dp.Exemplars())
			if exemplarErr != nil {
				return exemplarErr
			}
			appendErr := batch.Append(
				resAttrBytes,
				resAttrKeys,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrBytes,
				scopeAttrKeys,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				attrBytes,
				attrKeys,
				dp.StartTimestamp().AsTime(),
				dp.Timestamp().AsTime(),
				getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()),
				uint32(dp.Flags()),
				exemplarAttrs,
				exemplarTimes,
				exemplarValues,
				exemplarSpanIDs,
				exemplarTraceIDs,
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append gauge json metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("gauge json metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert gauge json metrics",
		zap.Int("records", g.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

// --- sumMetricsJSON ---

type sumMetricsJSON struct {
	sumModels []*sumModel
	insertSQL string
	count     int
}

func (s *sumMetricsJSON) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
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

func (s *sumMetricsJSON) insert(ctx context.Context, db driver.Conn) error {
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
			logger.Warn("failed to close sum json metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range s.sumModels {
		resAttrBytes, resAttrKeys, err := marshalAttrs(model.metadata.ResAttr)
		if err != nil {
			return fmt.Errorf("failed to marshal sum resource attributes: %w", err)
		}
		scopeAttrBytes, scopeAttrKeys, err := marshalAttrs(model.metadata.ScopeInstr.Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal sum scope attributes: %w", err)
		}
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.sum.DataPoints().Len(); i++ {
			dp := model.sum.DataPoints().At(i)
			attrBytes, attrKeys, err := marshalAttrs(dp.Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal sum data point attributes: %w", err)
			}
			exemplarAttrs, exemplarTimes, exemplarValues, exemplarTraceIDs, exemplarSpanIDs, exemplarErr := convertExemplarsJSON(dp.Exemplars())
			if exemplarErr != nil {
				return exemplarErr
			}
			appendErr := batch.Append(
				resAttrBytes,
				resAttrKeys,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrBytes,
				scopeAttrKeys,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				attrBytes,
				attrKeys,
				dp.StartTimestamp().AsTime(),
				dp.Timestamp().AsTime(),
				getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()),
				uint32(dp.Flags()),
				exemplarAttrs,
				exemplarTimes,
				exemplarValues,
				exemplarSpanIDs,
				exemplarTraceIDs,
				int32(model.sum.AggregationTemporality()),
				model.sum.IsMonotonic(),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append sum json metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("sum json metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert sum json metrics",
		zap.Int("records", s.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

// --- histogramMetricsJSON ---

type histogramMetricsJSON struct {
	histogramModels []*histogramModel
	insertSQL       string
	count           int
}

func (h *histogramMetricsJSON) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
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

func (h *histogramMetricsJSON) insert(ctx context.Context, db driver.Conn) error {
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
			logger.Warn("failed to close histogram json metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range h.histogramModels {
		resAttrBytes, resAttrKeys, err := marshalAttrs(model.metadata.ResAttr)
		if err != nil {
			return fmt.Errorf("failed to marshal histogram resource attributes: %w", err)
		}
		scopeAttrBytes, scopeAttrKeys, err := marshalAttrs(model.metadata.ScopeInstr.Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal histogram scope attributes: %w", err)
		}
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.histogram.DataPoints().Len(); i++ {
			dp := model.histogram.DataPoints().At(i)
			attrBytes, attrKeys, err := marshalAttrs(dp.Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal histogram data point attributes: %w", err)
			}
			exemplarAttrs, exemplarTimes, exemplarValues, exemplarTraceIDs, exemplarSpanIDs, exemplarErr := convertExemplarsJSON(dp.Exemplars())
			if exemplarErr != nil {
				return exemplarErr
			}
			appendErr := batch.Append(
				resAttrBytes,
				resAttrKeys,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrBytes,
				scopeAttrKeys,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				attrBytes,
				attrKeys,
				dp.StartTimestamp().AsTime(),
				dp.Timestamp().AsTime(),
				dp.Count(),
				dp.Sum(),
				convertSliceToArraySet(dp.BucketCounts().AsRaw()),
				convertSliceToArraySet(dp.ExplicitBounds().AsRaw()),
				exemplarAttrs,
				exemplarTimes,
				exemplarValues,
				exemplarSpanIDs,
				exemplarTraceIDs,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.histogram.AggregationTemporality()),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append histogram json metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("histogram json metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert histogram json metrics",
		zap.Int("records", h.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

// --- expHistogramMetricsJSON ---

type expHistogramMetricsJSON struct {
	expHistogramModels []*expHistogramModel
	insertSQL          string
	count              int
}

func (e *expHistogramMetricsJSON) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
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

func (e *expHistogramMetricsJSON) insert(ctx context.Context, db driver.Conn) error {
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
			logger.Warn("failed to close exponential histogram json metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range e.expHistogramModels {
		resAttrBytes, resAttrKeys, err := marshalAttrs(model.metadata.ResAttr)
		if err != nil {
			return fmt.Errorf("failed to marshal exponential histogram resource attributes: %w", err)
		}
		scopeAttrBytes, scopeAttrKeys, err := marshalAttrs(model.metadata.ScopeInstr.Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal exponential histogram scope attributes: %w", err)
		}
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.expHistogram.DataPoints().Len(); i++ {
			dp := model.expHistogram.DataPoints().At(i)
			attrBytes, attrKeys, err := marshalAttrs(dp.Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal exponential histogram data point attributes: %w", err)
			}
			exemplarAttrs, exemplarTimes, exemplarValues, exemplarTraceIDs, exemplarSpanIDs, exemplarErr := convertExemplarsJSON(dp.Exemplars())
			if exemplarErr != nil {
				return exemplarErr
			}
			appendErr := batch.Append(
				resAttrBytes,
				resAttrKeys,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrBytes,
				scopeAttrKeys,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				attrBytes,
				attrKeys,
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
				exemplarAttrs,
				exemplarTimes,
				exemplarValues,
				exemplarSpanIDs,
				exemplarTraceIDs,
				uint32(dp.Flags()),
				dp.Min(),
				dp.Max(),
				int32(model.expHistogram.AggregationTemporality()),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append exponential histogram json metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("exponential histogram json metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert exponential histogram json metrics",
		zap.Int("records", e.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

// --- summaryMetricsJSON ---

type summaryMetricsJSON struct {
	summaryModels []*summaryModel
	insertSQL     string
	count         int
}

func (s *summaryMetricsJSON) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
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

func (s *summaryMetricsJSON) insert(ctx context.Context, db driver.Conn) error {
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
			logger.Warn("failed to close summary json metrics batch", zap.Error(closeErr))
		}
	}(batch)

	for _, model := range s.summaryModels {
		resAttrBytes, resAttrKeys, err := marshalAttrs(model.metadata.ResAttr)
		if err != nil {
			return fmt.Errorf("failed to marshal summary resource attributes: %w", err)
		}
		scopeAttrBytes, scopeAttrKeys, err := marshalAttrs(model.metadata.ScopeInstr.Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal summary scope attributes: %w", err)
		}
		serviceName := GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.summary.DataPoints().Len(); i++ {
			dp := model.summary.DataPoints().At(i)
			attrBytes, attrKeys, err := marshalAttrs(dp.Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal summary data point attributes: %w", err)
			}
			quantiles, values := convertValueAtQuantile(dp.QuantileValues())
			appendErr := batch.Append(
				resAttrBytes,
				resAttrKeys,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrBytes,
				scopeAttrKeys,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				serviceName,
				model.metricName,
				model.metricDescription,
				model.metricUnit,
				attrBytes,
				attrKeys,
				dp.StartTimestamp().AsTime(),
				dp.Timestamp().AsTime(),
				dp.Count(),
				dp.Sum(),
				quantiles,
				values,
				uint32(dp.Flags()),
			)
			if appendErr != nil {
				return fmt.Errorf("failed to append summary json metric: %w", appendErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("summary json metric insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	logger.Debug("insert summary json metrics",
		zap.Int("records", s.count),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}
