// Copyright  The OpenTelemetry Authors
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

package metadata

import "go.opentelemetry.io/collector/model/pdata"

type MetricDataType interface {
	MetricDataType() pdata.MetricDataType
	AggregationTemporality() pdata.MetricAggregationTemporality
	IsMonotonic() bool
}

type metricValueDataType struct {
	dataType               pdata.MetricDataType
	aggregationTemporality pdata.MetricAggregationTemporality
	isMonotonic            bool
}

func NewMetricDataType(dataType pdata.MetricDataType, aggregationTemporality pdata.MetricAggregationTemporality,
	isMonotonic bool) MetricDataType {
	return metricValueDataType{
		dataType:               dataType,
		aggregationTemporality: aggregationTemporality,
		isMonotonic:            isMonotonic,
	}
}

func (metricValueDataType metricValueDataType) MetricDataType() pdata.MetricDataType {
	return metricValueDataType.dataType
}

func (metricValueDataType metricValueDataType) AggregationTemporality() pdata.MetricAggregationTemporality {
	return metricValueDataType.aggregationTemporality
}

func (metricValueDataType metricValueDataType) IsMonotonic() bool {
	return metricValueDataType.isMonotonic
}
