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

func (metricType MetricType) dataType() (pmetric.MetricDataType, error) {
	var dataType pmetric.MetricDataType

	switch metricType.DataType {
	case GaugeMetricDataType:
		dataType = pmetric.MetricDataTypeGauge
	case SumMetricDataType:
		dataType = pmetric.MetricDataTypeSum
	default:
		return pmetric.MetricDataTypeNone, errors.New("invalid data type received")
	}

	return dataType, nil
}

func (metricType MetricType) aggregationTemporality() (pmetric.MetricAggregationTemporality, error) {
	var aggregationTemporality pmetric.MetricAggregationTemporality

	switch metricType.Aggregation {
	case DeltaAggregationType:
		aggregationTemporality = pmetric.MetricAggregationTemporalityDelta
	case CumulativeAggregationType:
		aggregationTemporality = pmetric.MetricAggregationTemporalityCumulative
	case "":
		aggregationTemporality = pmetric.MetricAggregationTemporalityUnspecified
	default:
		return pmetric.MetricAggregationTemporalityUnspecified, errors.New("invalid aggregation temporality received")
	}

	return aggregationTemporality, nil
}

func (metricType MetricType) toMetricDataType() (metadata.MetricDataType, error) {
	dataType, err := metricType.dataType()
	if err != nil {
		return nil, err
	}

	aggregationTemporality, err := metricType.aggregationTemporality()
	if err != nil {
		return nil, err
	}

	return metadata.NewMetricDataType(dataType, aggregationTemporality, metricType.Monotonic), nil
}
