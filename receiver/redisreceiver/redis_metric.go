// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
)

// An intermediate data type that allows us to define at startup which metrics to
// convert (from the string-string map we get from redisSvc) and how to convert them.
type redisMetric struct {
	key         string
	name        string
	units       string
	desc        string
	labels      map[string]pdata.AttributeValue
	pdType      pdata.MetricDataType
	valueType   pdata.MetricValueType
	isMonotonic bool
}

// Parse a numeric string to build a metric based on this redisMetric. The
// passed-in time is applied to the Point.
func (m *redisMetric) parseMetric(strVal string, t *timeBundle) (pdata.Metric, error) {
	pdm := pdata.NewMetric()
	switch m.pdType {
	case pdata.MetricDataTypeSum:
		switch m.valueType {
		case pdata.MetricValueTypeDouble:
			val, err := strToDoublePoint(strVal)
			if err != nil {
				return pdm, err
			}
			initDoubleMetric(m, val, t, pdm)
		case pdata.MetricValueTypeInt:
			val, err := strToInt64Point(strVal)
			if err != nil {
				return pdm, err
			}
			initIntMetric(m, val, t, pdm)
		}
	case pdata.MetricDataTypeGauge:
		switch m.valueType {
		case pdata.MetricValueTypeDouble:
			val, err := strToDoublePoint(strVal)
			if err != nil {
				return pdm, err
			}
			initDoubleMetric(m, val, t, pdm)
		case pdata.MetricValueTypeInt:
			val, err := strToInt64Point(strVal)
			if err != nil {
				return pdm, err
			}
			initIntMetric(m, val, t, pdm)
		}
	}
	return pdm, nil
}

// Converts a numeric whole number string to a Point.
func strToInt64Point(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// Converts a numeric floating point string to a Point.
func strToDoublePoint(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
