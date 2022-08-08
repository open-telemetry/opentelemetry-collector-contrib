// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func rowToMetric(row metricRow, cfg MetricCfg, dest pmetric.Metric) error {
	dest.SetName(cfg.MetricName)
	dest.SetDescription(cfg.Description)
	dest.SetUnit(cfg.Unit)
	dataPointSlice := setMetricFields(cfg, dest)
	dataPoint := dataPointSlice.AppendEmpty()
	value, found := row[cfg.ValueColumn]
	if !found {
		return fmt.Errorf("rowToMetric: value_column '%s' not found in result set", cfg.ValueColumn)
	}
	err := setDataPointValue(cfg, value, dataPoint)
	if err != nil {
		return fmt.Errorf("rowToMetric: %w", err)
	}
	attrs := dataPoint.Attributes()
	for _, columnName := range cfg.AttributeColumns {
		if attrVal, found := row[columnName]; found {
			attrs.InsertString(columnName, attrVal)
		} else {
			return fmt.Errorf("rowToMetric: attribute_column not found: '%s'", columnName)
		}
	}
	return nil
}

func setMetricFields(cfg MetricCfg, dest pmetric.Metric) pmetric.NumberDataPointSlice {
	var out pmetric.NumberDataPointSlice
	switch cfg.DataType {
	case MetricDataTypeUnspecified, MetricDataTypeGauge:
		dest.SetDataType(pmetric.MetricDataTypeGauge)
		out = dest.Gauge().DataPoints()
	case MetricDataTypeSum:
		dest.SetDataType(pmetric.MetricDataTypeSum)
		sum := dest.Sum()
		sum.SetIsMonotonic(cfg.Monotonic)
		sum.SetAggregationTemporality(cfgToAggregationTemporality(cfg.Aggregation))
		out = sum.DataPoints()
	}
	return out
}

func cfgToAggregationTemporality(agg MetricAggregation) pmetric.MetricAggregationTemporality {
	var out pmetric.MetricAggregationTemporality
	switch agg {
	case MetricAggregationUnspecified, MetricAggregationCumulative:
		out = pmetric.MetricAggregationTemporalityCumulative
	case MetricAggregationDelta:
		out = pmetric.MetricAggregationTemporalityDelta
	}
	return out
}

func setDataPointValue(cfg MetricCfg, str string, dest pmetric.NumberDataPoint) error {
	switch cfg.ValueType {
	case MetricValueTypeUnspecified, MetricValueTypeInt:
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("setDataPointValue: error converting to integer: %w", err)
		}
		dest.SetIntVal(int64(val))
	case MetricValueTypeDouble:
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("setDataPointValue: error converting to double: %w", err)
		}
		dest.SetDoubleVal(val)
	}
	return nil
}
