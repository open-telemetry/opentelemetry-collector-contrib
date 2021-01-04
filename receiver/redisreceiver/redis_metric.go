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

	"go.opentelemetry.io/collector/consumer/pdata"
)

// An intermediate data type that allows us to define at startup which metrics to
// convert (from the string-string map we get from redisSvc) and how to convert them.
type redisMetric struct {
	key         string
	name        string
	units       string
	desc        string
	labels      map[string]string
	pdType      pdata.MetricDataType
	isMonotonic bool
}

// Parse a numeric string to build a metric based on this redisMetric. The
// passed-in time is applied to the Point.
func (m *redisMetric) parseMetric(strVal string, t *timeBundle) (pdata.Metric, error) {
	var err error
	var pdm pdata.Metric
	switch m.pdType {
	case pdata.MetricDataTypeIntSum:
		var pt pdata.IntDataPoint
		pt, err = strToInt64Point(strVal)
		if err != nil {
			return pdata.Metric{}, err
		}
		pdm = newIntMetric(m, pt, t)
	case pdata.MetricDataTypeIntGauge:
		var pt pdata.IntDataPoint
		pt, err = strToInt64Point(strVal)
		if err != nil {
			return pdata.Metric{}, err
		}
		pdm = newIntMetric(m, pt, t)
	case pdata.MetricDataTypeDoubleSum:
		var pt pdata.DoubleDataPoint
		pt, err = strToDoublePoint(strVal)
		if err != nil {
			return pdata.Metric{}, err
		}
		pdm = newDoubleMetric(m, pt, t)
	case pdata.MetricDataTypeDoubleGauge:
		var pt pdata.DoubleDataPoint
		pt, err = strToDoublePoint(strVal)
		if err != nil {
			return pdata.Metric{}, err
		}
		pdm = newDoubleMetric(m, pt, t)
	}
	return pdm, nil
}

// Converts a numeric whole number string to a Point.
func strToInt64Point(s string) (pdata.IntDataPoint, error) {
	pt := pdata.NewIntDataPoint()
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return pt, err
	}
	pt.SetValue(i)
	return pt, nil
}

// Converts a numeric floating point string to a Point.
func strToDoublePoint(s string) (pdata.DoubleDataPoint, error) {
	pt := pdata.NewDoubleDataPoint()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return pt, err
	}
	pt.SetValue(f)
	return pt, nil
}
