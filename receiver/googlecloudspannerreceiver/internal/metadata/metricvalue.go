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

type MetricValueMetadata interface {
	ValueMetadata
	DataType() MetricDataType
	Unit() string
}

type MetricValue interface {
	MetricValueMetadata
	Value() interface{}
}

type queryMetricValueMetadata struct {
	name       string
	columnName string
	dataType   MetricDataType
	unit       string
}

func newQueryMetricValueMetadata(name string, columnName string, dataType MetricDataType,
	unit string) queryMetricValueMetadata {

	return queryMetricValueMetadata{
		name:       name,
		columnName: columnName,
		dataType:   dataType,
		unit:       unit,
	}
}

type Int64MetricValueMetadata struct {
	queryMetricValueMetadata
}

func NewInt64MetricValueMetadata(name string, columnName string, dataType MetricDataType,
	unit string) Int64MetricValueMetadata {

	return Int64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata(name, columnName, dataType, unit),
	}
}

type Float64MetricValueMetadata struct {
	queryMetricValueMetadata
}

func NewFloat64MetricValueMetadata(name string, columnName string, dataType MetricDataType,
	unit string) Float64MetricValueMetadata {

	return Float64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata(name, columnName, dataType, unit),
	}
}

type int64MetricValue struct {
	Int64MetricValueMetadata
	value int64
}

type float64MetricValue struct {
	Float64MetricValueMetadata
	value float64
}

func (metadata queryMetricValueMetadata) Name() string {
	return metadata.name
}

func (metadata queryMetricValueMetadata) ColumnName() string {
	return metadata.columnName
}

func (metadata queryMetricValueMetadata) DataType() MetricDataType {
	return metadata.dataType
}

func (metadata queryMetricValueMetadata) Unit() string {
	return metadata.unit
}

func (metadata Int64MetricValueMetadata) ValueHolder() interface{} {
	var valueHolder int64

	return &valueHolder
}

func (metadata Float64MetricValueMetadata) ValueHolder() interface{} {
	var valueHolder float64

	return &valueHolder
}

func (value int64MetricValue) Value() interface{} {
	return value.value
}

func (value float64MetricValue) Value() interface{} {
	return value.value
}

func newInt64MetricValue(metadata Int64MetricValueMetadata, valueHolder interface{}) int64MetricValue {
	return int64MetricValue{
		Int64MetricValueMetadata: metadata,
		value:                    *valueHolder.(*int64),
	}
}

func newFloat64MetricValue(metadata Float64MetricValueMetadata, valueHolder interface{}) float64MetricValue {
	return float64MetricValue{
		Float64MetricValueMetadata: metadata,
		value:                      *valueHolder.(*float64),
	}
}
