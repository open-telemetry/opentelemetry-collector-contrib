// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func rowToMetric(row StringMap, cfg MetricCfg, dest pmetric.Metric, startTime pcommon.Timestamp, ts pcommon.Timestamp, scrapeCfg scraperhelper.ControllerConfig) error {
	dest.SetName(cfg.MetricName)
	dest.SetDescription(cfg.Description)
	dest.SetUnit(cfg.Unit)
	dataPointSlice := setMetricFields(cfg, dest)
	dataPoint := dataPointSlice.AppendEmpty()
	var errs []error
	if cfg.StartTsColumn != "" {
		if val, found := row[cfg.StartTsColumn]; found {
			timestamp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to parse uint64 for %q, value was %q: %w", cfg.StartTsColumn, val, err))
			}
			startTime = pcommon.Timestamp(timestamp)
		} else {
			errs = append(errs, fmt.Errorf("rowToMetric: start_ts_column not found"))
		}
	}
	if cfg.TsColumn != "" {
		if val, found := row[cfg.TsColumn]; found {
			timestamp, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to parse uint64 for %q, value was %q: %w", cfg.TsColumn, val, err))
			}
			ts = pcommon.Timestamp(timestamp)
		} else {
			errs = append(errs, fmt.Errorf("rowToMetric: ts_column not found"))
		}
	}
	setTimestamp(cfg, dataPoint, startTime, ts, scrapeCfg)
	value, found := row[cfg.ValueColumn]
	if !found {
		errs = append(errs, fmt.Errorf("rowToMetric: value_column '%s' not found in result set", cfg.ValueColumn))
	}

	err := setDataPointValue(cfg, value, dataPoint)
	if err != nil {
		errs = append(errs, fmt.Errorf("rowToMetric: %w", err))
	}
	attrs := dataPoint.Attributes()
	for k, v := range cfg.StaticAttributes {
		attrs.PutStr(k, v)
	}
	for _, columnName := range cfg.AttributeColumns {
		if attrVal, found := row[columnName]; found {
			attrs.PutStr(columnName, attrVal)
		} else {
			errs = append(errs, fmt.Errorf("rowToMetric: attribute_column '%s' not found in result set", columnName))
		}
	}
	return errors.Join(errs...)
}

func setTimestamp(cfg MetricCfg, dp pmetric.NumberDataPoint, startTime pcommon.Timestamp, ts pcommon.Timestamp, scrapeCfg scraperhelper.ControllerConfig) {
	dp.SetTimestamp(ts)

	// Cumulative sum should have a start time set to the beginning of the data points cumulation
	if cfg.Aggregation == MetricAggregationCumulative && cfg.DataType != MetricTypeGauge {
		dp.SetStartTimestamp(startTime)
	}

	// Non-cumulative sum should have a start time set to the previous endpoint
	if cfg.Aggregation == MetricAggregationDelta && cfg.DataType != MetricTypeGauge {
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(ts.AsTime().Add(-scrapeCfg.CollectionInterval)))
	}
}

func setMetricFields(cfg MetricCfg, dest pmetric.Metric) pmetric.NumberDataPointSlice {
	var out pmetric.NumberDataPointSlice
	switch cfg.DataType {
	case MetricTypeUnspecified, MetricTypeGauge:
		out = dest.SetEmptyGauge().DataPoints()
	case MetricTypeSum:
		sum := dest.SetEmptySum()
		sum.SetIsMonotonic(cfg.Monotonic)
		sum.SetAggregationTemporality(cfgToAggregationTemporality(cfg.Aggregation))
		out = sum.DataPoints()
	}
	return out
}

func cfgToAggregationTemporality(agg MetricAggregation) pmetric.AggregationTemporality {
	var out pmetric.AggregationTemporality
	switch agg {
	case MetricAggregationUnspecified, MetricAggregationCumulative:
		out = pmetric.AggregationTemporalityCumulative
	case MetricAggregationDelta:
		out = pmetric.AggregationTemporalityDelta
	}
	return out
}

func setDataPointValue(cfg MetricCfg, str string, dest pmetric.NumberDataPoint) error {
	switch cfg.ValueType {
	case MetricValueTypeUnspecified, MetricValueTypeInt:
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("setDataPointValue: col %q: error converting to integer: %w", cfg.ValueColumn, err)
		}
		dest.SetIntValue(int64(val))
	case MetricValueTypeDouble:
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("setDataPointValue: col %q: error converting to double: %w", cfg.ValueColumn, err)
		}
		dest.SetDoubleValue(val)
	}
	return nil
}
