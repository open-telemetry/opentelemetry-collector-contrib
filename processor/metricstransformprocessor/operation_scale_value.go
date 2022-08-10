// Copyright 2021 OpenTelemetry Authors
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

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (mtp *metricsTransformProcessor) scaleValueOp(metric *metricspb.Metric, op internalOperation) {
	for _, ts := range metric.Timeseries {
		for _, dp := range ts.Points {
			switch metric.MetricDescriptor.Type {
			case metricspb.MetricDescriptor_GAUGE_INT64, metricspb.MetricDescriptor_CUMULATIVE_INT64:
				dp.Value = &metricspb.Point_Int64Value{Int64Value: int64(float64(dp.GetInt64Value()) * op.configOperation.Scale)}
			case metricspb.MetricDescriptor_GAUGE_DOUBLE, metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
				dp.Value = &metricspb.Point_DoubleValue{DoubleValue: dp.GetDoubleValue() * op.configOperation.Scale}
			}
		}
	}
}

// scaleValueOp scales a numeric metric value. Applicable to sum and gauge metrics only.
func scaleValueOp(metric pmetric.Metric, op internalOperation, f internalFilter) {
	var dps pmetric.NumberDataPointSlice
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricDataTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		if !f.matchAttrs(dp.Attributes()) {
			continue
		}
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			dp.SetIntVal(int64(float64(dp.IntVal()) * op.configOperation.Scale))
		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetDoubleVal(dp.DoubleVal() * op.configOperation.Scale)
		}
	}
}
