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
	"strings"
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
	HasNumberDataPoints() bool
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
		return "pdata.MetricAggregationTemporalityDelta"
	case "cumulative":
		return "pdata.MetricAggregationTemporalityCumulative"
	default:
		return "pdata.MetricAggregationTemporalityUnknown"
	}
}

// Mono defines the metric monotonicity.
type Mono struct {
	// Monotonic is true if the sum is monotonic.
	Monotonic bool `mapstructure:"monotonic"`
}

// NumberDataPoints defines the metric number type.
type NumberDataPoints struct {
	// Type is type of the metric number, options are "double", "int".
	// TODO: Add validation once the metric number type added to all metadata files.
	NumberType string `mapstructure:"number_type"`
}

// Type returns name of the datapoint type.
func (ndp NumberDataPoints) Type() string {
	return strings.Title(ndp.NumberType)
}

// BasicType returns name of a golang basic type for the datapoint type.
func (ndp NumberDataPoints) BasicType() string {
	switch ndp.NumberType {
	case "int":
		return "int64"
	case "double":
		return "float64"
	default:
		panic(fmt.Sprintf("unknown number data point type: %v", ndp.NumberType))
	}
}

type gauge struct {
	NumberDataPoints `yaml:",inline"`
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

func (d gauge) HasNumberDataPoints() bool {
	return true
}

type sum struct {
	Aggregated       `mapstructure:",squash"`
	Mono             `mapstructure:",squash"`
	NumberDataPoints `mapstructure:",squash"`
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

func (d sum) HasNumberDataPoints() bool {
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

func (d histogram) HasNumberDataPoints() bool {
	return false
}
