// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type MetricDataType string

const (
	UnknownMetricDataType MetricDataType = "unknown"
	GaugeMetricDataType   MetricDataType = "gauge"
	SumMetricDataType     MetricDataType = "sum"
)

type AggregationType string

const (
	UnknownAggregationType    AggregationType = "unknown"
	DeltaAggregationType      AggregationType = "delta"
	CumulativeAggregationType AggregationType = "cumulative"
)

type MetricType struct {
	DataType    MetricDataType  `yaml:"type"`
	Aggregation AggregationType `yaml:"aggregation"`
	Monotonic   bool            `yaml:"monotonic"`
}

func (metricType MetricType) dataType() (pmetric.MetricType, error) {
	var dataType pmetric.MetricType

	switch metricType.DataType {
	case GaugeMetricDataType:
		dataType = pmetric.MetricTypeGauge
	case SumMetricDataType:
		dataType = pmetric.MetricTypeSum
	default:
		return pmetric.MetricTypeEmpty, errors.New("invalid data type received")
	}

	return dataType, nil
}

func (metricType MetricType) aggregationTemporality() (pmetric.AggregationTemporality, error) {
	var aggregationTemporality pmetric.AggregationTemporality

	switch metricType.Aggregation {
	case DeltaAggregationType:
		aggregationTemporality = pmetric.AggregationTemporalityDelta
	case CumulativeAggregationType:
		aggregationTemporality = pmetric.AggregationTemporalityCumulative
	case "":
		aggregationTemporality = pmetric.AggregationTemporalityUnspecified
	default:
		return pmetric.AggregationTemporalityUnspecified, errors.New("invalid aggregation temporality received")
	}

	return aggregationTemporality, nil
}

func (metricType MetricType) toMetricType() (metadata.MetricType, error) {
	dataType, err := metricType.dataType()
	if err != nil {
		return nil, err
	}

	aggregationTemporality, err := metricType.aggregationTemporality()
	if err != nil {
		return nil, err
	}

	return metadata.NewMetricType(dataType, aggregationTemporality, metricType.Monotonic), nil
}
