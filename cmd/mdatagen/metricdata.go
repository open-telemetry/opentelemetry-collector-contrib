// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	_ MetricData = &gauge{}
	_ MetricData = &sum{}
	_ MetricData = &histogram{}
)

// MetricData is generic interface for all metric datatypes.
type MetricData interface {
	Type() string
	HasMonotonic() bool
	HasAggregated() bool
	HasMetricValueType() bool
}

// Aggregated defines a metric aggregation type.
type Aggregated struct {
	// Aggregation describes if the aggregator reports delta changes
	// since last report time, or cumulative changes since a fixed start time.
	Aggregation string `mapstructure:"aggregation" validate:"oneof=delta cumulative"`
}

// Type gets the metric aggregation type.
func (agg Aggregated) Type() string {
	switch agg.Aggregation {
	case "delta":
		return "pmetric.MetricAggregationTemporalityDelta"
	case "cumulative":
		return "pmetric.MetricAggregationTemporalityCumulative"
	default:
		return "pmetric.MetricAggregationTemporalityUnknown"
	}
}

// Mono defines the metric monotonicity.
type Mono struct {
	// Monotonic is true if the sum is monotonic.
	Monotonic bool `mapstructure:"monotonic"`
}

// MetricValueType defines the metric number type.
type MetricValueType struct {
	// ValueType is type of the metric number, options are "double", "int".
	ValueType pmetric.MetricValueType `validate:"required"`
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (mvt *MetricValueType) UnmarshalText(text []byte) error {
	switch vtStr := string(text); vtStr {
	case "int":
		mvt.ValueType = pmetric.MetricValueTypeInt
	case "double":
		mvt.ValueType = pmetric.MetricValueTypeDouble
	default:
		return fmt.Errorf("invalid value_type: %q", vtStr)
	}
	return nil
}

// Type returns name of the datapoint type.
func (mvt MetricValueType) String() string {
	return mvt.ValueType.String()
}

// BasicType returns name of a golang basic type for the datapoint type.
func (mvt MetricValueType) BasicType() string {
	switch mvt.ValueType {
	case pmetric.MetricValueTypeInt:
		return "int64"
	case pmetric.MetricValueTypeDouble:
		return "float64"
	default:
		return ""
	}
}

type gauge struct {
	MetricValueType `mapstructure:"value_type"`
}

func (d gauge) Type() string {
	return "Gauge"
}

func (d gauge) HasMonotonic() bool {
	return false
}

func (d gauge) HasAggregated() bool {
	return false
}

func (d gauge) HasMetricValueType() bool {
	return true
}

type sum struct {
	Aggregated      `mapstructure:",squash"`
	Mono            `mapstructure:",squash"`
	MetricValueType `mapstructure:"value_type"`
}

func (d sum) Type() string {
	return "Sum"
}

func (d sum) HasMonotonic() bool {
	return true
}

func (d sum) HasAggregated() bool {
	return true
}

func (d sum) HasMetricValueType() bool {
	return true
}

type histogram struct {
	Aggregated `mapstructure:",squash"`
}

func (d histogram) Type() string {
	return "Histogram"
}

func (d histogram) HasMonotonic() bool {
	return false
}

func (d histogram) HasAggregated() bool {
	return true
}

func (d histogram) HasMetricValueType() bool {
	return false
}
