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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pmetric"

type MetricDataType interface {
	MetricDataType() pmetric.MetricDataType
	AggregationTemporality() pmetric.MetricAggregationTemporality
	IsMonotonic() bool
}

type metricValueDataType struct {
	dataType               pmetric.MetricDataType
	aggregationTemporality pmetric.MetricAggregationTemporality
	isMonotonic            bool
}

func NewMetricDataType(dataType pmetric.MetricDataType, aggregationTemporality pmetric.MetricAggregationTemporality,
	isMonotonic bool) MetricDataType {
	return metricValueDataType{
		dataType:               dataType,
		aggregationTemporality: aggregationTemporality,
		isMonotonic:            isMonotonic,
	}
}

func (metricValueDataType metricValueDataType) MetricDataType() pmetric.MetricDataType {
	return metricValueDataType.dataType
}

func (metricValueDataType metricValueDataType) AggregationTemporality() pmetric.MetricAggregationTemporality {
	return metricValueDataType.aggregationTemporality
}

func (metricValueDataType metricValueDataType) IsMonotonic() bool {
	return metricValueDataType.isMonotonic
}
