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

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/serialization"

import (
	"fmt"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/model/pdata"
)

func serializeGauge(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint) (string, error) {
	var metricOption dtMetric.MetricOption

	switch dp.ValueType() {
	case pdata.MetricValueTypeNone:
		return "", fmt.Errorf("unsupported value type none")
	case pdata.MetricValueTypeInt:
		metricOption = dtMetric.WithIntGaugeValue(dp.IntVal())
	case pdata.MetricValueTypeDouble:
		metricOption = dtMetric.WithFloatGaugeValue(dp.DoubleVal())
	default:
		return "", fmt.Errorf("unknown data type")
	}

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		metricOption,
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}
