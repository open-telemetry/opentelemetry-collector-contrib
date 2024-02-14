// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pmetric"

type MetricType interface {
	MetricType() pmetric.MetricType
	AggregationTemporality() pmetric.AggregationTemporality
	IsMonotonic() bool
}

type metricValueDataType struct {
	dataType               pmetric.MetricType
	aggregationTemporality pmetric.AggregationTemporality
	isMonotonic            bool
}

func NewMetricType(dataType pmetric.MetricType, aggregationTemporality pmetric.AggregationTemporality,
	isMonotonic bool) MetricType {
	return metricValueDataType{
		dataType:               dataType,
		aggregationTemporality: aggregationTemporality,
		isMonotonic:            isMonotonic,
	}
}

func (metricValueDataType metricValueDataType) MetricType() pmetric.MetricType {
	return metricValueDataType.dataType
}

func (metricValueDataType metricValueDataType) AggregationTemporality() pmetric.AggregationTemporality {
	return metricValueDataType.aggregationTemporality
}

func (metricValueDataType metricValueDataType) IsMonotonic() bool {
	return metricValueDataType.isMonotonic
}
