// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type MetricContext interface {
	GetMetric() pmetric.Metric
}

var MetricSymbolTable = map[ottl.EnumSymbol]ottl.Enum{
	"AGGREGATION_TEMPORALITY_UNSPECIFIED":    ottl.Enum(pmetric.AggregationTemporalityUnspecified),
	"AGGREGATION_TEMPORALITY_DELTA":          ottl.Enum(pmetric.AggregationTemporalityDelta),
	"AGGREGATION_TEMPORALITY_CUMULATIVE":     ottl.Enum(pmetric.AggregationTemporalityCumulative),
	"METRIC_DATA_TYPE_NONE":                  ottl.Enum(pmetric.MetricTypeEmpty),
	"METRIC_DATA_TYPE_GAUGE":                 ottl.Enum(pmetric.MetricTypeGauge),
	"METRIC_DATA_TYPE_SUM":                   ottl.Enum(pmetric.MetricTypeSum),
	"METRIC_DATA_TYPE_HISTOGRAM":             ottl.Enum(pmetric.MetricTypeHistogram),
	"METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM": ottl.Enum(pmetric.MetricTypeExponentialHistogram),
	"METRIC_DATA_TYPE_SUMMARY":               ottl.Enum(pmetric.MetricTypeSummary),
}

func MetricPathGetSetter[K MetricContext](path []ottl.Field) (ottl.GetSetter[K], error) {
	if len(path) == 0 {
		return accessMetric[K](), nil
	}
	switch path[0].Name {
	case "name":
		return accessName[K](), nil
	case "description":
		return accessDescription[K](), nil
	case "unit":
		return accessUnit[K](), nil
	case "type":
		return accessType[K](), nil
	case "aggregation_temporality":
		return accessAggTemporality[K](), nil
	case "is_monotonic":
		return accessIsMonotonic[K](), nil
	case "data_points":
		return accessDataPoints[K](), nil
	}

	return nil, fmt.Errorf("invalid metric path expression %v", path)
}

func accessMetric[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetMetric()
		},
		Setter: func(ctx K, val interface{}) {
			if newMetric, ok := val.(pmetric.Metric); ok {
				newMetric.CopyTo(ctx.GetMetric())
			}
		},
	}
}

func accessName[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetMetric().Name()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetName(str)
			}
		},
	}
}

func accessDescription[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetMetric().Description()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetDescription(str)
			}
		},
	}
}

func accessUnit[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetMetric().Unit()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetUnit(str)
			}
		},
	}
}

func accessType[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetMetric().Type())
		},
		Setter: func(ctx K, val interface{}) {
			// TODO Implement methods so correctly convert data types.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10130
		},
	}
}

func accessAggTemporality[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			metric := ctx.GetMetric()
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				return int64(metric.Sum().AggregationTemporality())
			case pmetric.MetricTypeHistogram:
				return int64(metric.Histogram().AggregationTemporality())
			case pmetric.MetricTypeExponentialHistogram:
				return int64(metric.ExponentialHistogram().AggregationTemporality())
			}
			return nil
		},
		Setter: func(ctx K, val interface{}) {
			if newAggTemporality, ok := val.(int64); ok {
				metric := ctx.GetMetric()
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporality(newAggTemporality))
				case pmetric.MetricTypeHistogram:
					metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporality(newAggTemporality))
				case pmetric.MetricTypeExponentialHistogram:
					metric.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporality(newAggTemporality))
				}
			}
		},
	}
}

func accessIsMonotonic[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			metric := ctx.GetMetric()
			if metric.Type() == pmetric.MetricTypeSum {
				return metric.Sum().IsMonotonic()
			}
			return nil
		},
		Setter: func(ctx K, val interface{}) {
			if newIsMonotonic, ok := val.(bool); ok {
				metric := ctx.GetMetric()
				if metric.Type() == pmetric.MetricTypeSum {
					metric.Sum().SetIsMonotonic(newIsMonotonic)
				}
			}
		},
	}
}

func accessDataPoints[K MetricContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			metric := ctx.GetMetric()
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				return metric.Sum().DataPoints()
			case pmetric.MetricTypeGauge:
				return metric.Gauge().DataPoints()
			case pmetric.MetricTypeHistogram:
				return metric.Histogram().DataPoints()
			case pmetric.MetricTypeExponentialHistogram:
				return metric.ExponentialHistogram().DataPoints()
			case pmetric.MetricTypeSummary:
				return metric.Summary().DataPoints()
			}
			return nil
		},
		Setter: func(ctx K, val interface{}) {
			metric := ctx.GetMetric()
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				if newDataPoints, ok := val.(pmetric.NumberDataPointSlice); ok {
					newDataPoints.CopyTo(metric.Sum().DataPoints())
				}
			case pmetric.MetricTypeGauge:
				if newDataPoints, ok := val.(pmetric.NumberDataPointSlice); ok {
					newDataPoints.CopyTo(metric.Gauge().DataPoints())
				}
			case pmetric.MetricTypeHistogram:
				if newDataPoints, ok := val.(pmetric.HistogramDataPointSlice); ok {
					newDataPoints.CopyTo(metric.Histogram().DataPoints())
				}
			case pmetric.MetricTypeExponentialHistogram:
				if newDataPoints, ok := val.(pmetric.ExponentialHistogramDataPointSlice); ok {
					newDataPoints.CopyTo(metric.ExponentialHistogram().DataPoints())
				}
			case pmetric.MetricTypeSummary:
				if newDataPoints, ok := val.(pmetric.SummaryDataPointSlice); ok {
					newDataPoints.CopyTo(metric.Summary().DataPoints())
				}
			}
		},
	}
}
