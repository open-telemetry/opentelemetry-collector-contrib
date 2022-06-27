// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"fmt"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func SerializeMetric(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions, staticDimensions dimensions.NormalizedDimensionList, prev *ttlmap.TTLMap) ([]string, error) {
	var metricLines []string

	ce := logger.Check(zap.DebugLevel, "SerializeMetric")
	var points int

	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		metricLines = serializeGauge(logger, prefix, metric, defaultDimensions, staticDimensions, metricLines)
	case pmetric.MetricDataTypeSum:
		metricLines = serializeSum(logger, prefix, metric, defaultDimensions, staticDimensions, prev, metricLines)
	case pmetric.MetricDataTypeHistogram:
		metricLines = serializeHistogram(logger, prefix, metric, defaultDimensions, staticDimensions, metricLines)
	default:
		return nil, fmt.Errorf("metric type %s unsupported", metric.DataType().String())
	}

	if ce != nil {
		ce.Write(zap.String("DataType", metric.DataType().String()), zap.Int("points", points))
	}

	return metricLines, nil
}

func makeCombinedDimensions(defaultDimensions dimensions.NormalizedDimensionList, dataPointAttributes pcommon.Map, staticDimensions dimensions.NormalizedDimensionList) dimensions.NormalizedDimensionList {
	dimsFromAttributes := make([]dimensions.Dimension, 0, dataPointAttributes.Len())

	dataPointAttributes.Range(func(k string, v pcommon.Value) bool {
		dimsFromAttributes = append(dimsFromAttributes, dimensions.NewDimension(k, v.AsString()))
		return true
	})
	return dimensions.MergeLists(
		defaultDimensions,
		dimensions.NewNormalizedDimensionList(dimsFromAttributes...),
		staticDimensions,
	)
}
