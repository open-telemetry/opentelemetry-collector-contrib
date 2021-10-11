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

import (
	"sort"
	"strings"
)

type LabelValueMetadata interface {
	ValueMetadata
}

type LabelValue interface {
	LabelValueMetadata
	Value() interface{}
}

type queryLabelValueMetadata struct {
	name       string
	columnName string
}

func newQueryLabelValueMetadata(name string, columnName string) queryLabelValueMetadata {
	return queryLabelValueMetadata{
		name:       name,
		columnName: columnName,
	}
}

type StringLabelValueMetadata struct {
	queryLabelValueMetadata
}

func NewStringLabelValueMetadata(name string, columnName string) StringLabelValueMetadata {
	return StringLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(name, columnName),
	}
}

type Int64LabelValueMetadata struct {
	queryLabelValueMetadata
}

func NewInt64LabelValueMetadata(name string, columnName string) Int64LabelValueMetadata {
	return Int64LabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(name, columnName),
	}
}

type BoolLabelValueMetadata struct {
	queryLabelValueMetadata
}

func NewBoolLabelValueMetadata(name string, columnName string) BoolLabelValueMetadata {
	return BoolLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(name, columnName),
	}
}

type StringSliceLabelValueMetadata struct {
	queryLabelValueMetadata
}

func NewStringSliceLabelValueMetadata(name string, columnName string) StringSliceLabelValueMetadata {
	return StringSliceLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(name, columnName),
	}
}

type ByteSliceLabelValueMetadata struct {
	queryLabelValueMetadata
}

func NewByteSliceLabelValueMetadata(name string, columnName string) ByteSliceLabelValueMetadata {
	return ByteSliceLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(name, columnName),
	}
}

type stringLabelValue struct {
	StringLabelValueMetadata
	value string
}

type int64LabelValue struct {
	Int64LabelValueMetadata
	value int64
}

type boolLabelValue struct {
	BoolLabelValueMetadata
	value bool
}

type stringSliceLabelValue struct {
	StringSliceLabelValueMetadata
	value string
}

type byteSliceLabelValue struct {
	ByteSliceLabelValueMetadata
	value string
}

func (metadata queryLabelValueMetadata) Name() string {
	return metadata.name
}

func (metadata queryLabelValueMetadata) ColumnName() string {
	return metadata.columnName
}

func (metadata StringLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder string

	return &valueHolder
}

func (value stringLabelValue) Value() interface{} {
	return value.value
}

func newStringLabelValue(metadata StringLabelValueMetadata, valueHolder interface{}) stringLabelValue {
	return stringLabelValue{
		StringLabelValueMetadata: metadata,
		value:                    *valueHolder.(*string),
	}
}

func (metadata Int64LabelValueMetadata) ValueHolder() interface{} {
	var valueHolder int64

	return &valueHolder
}

func (value int64LabelValue) Value() interface{} {
	return value.value
}

func newInt64LabelValue(metadata Int64LabelValueMetadata, valueHolder interface{}) int64LabelValue {
	return int64LabelValue{
		Int64LabelValueMetadata: metadata,
		value:                   *valueHolder.(*int64),
	}
}

func (metadata BoolLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder bool

	return &valueHolder
}

func (value boolLabelValue) Value() interface{} {
	return value.value
}

func newBoolLabelValue(metadata BoolLabelValueMetadata, valueHolder interface{}) boolLabelValue {
	return boolLabelValue{
		BoolLabelValueMetadata: metadata,
		value:                  *valueHolder.(*bool),
	}
}

func (metadata StringSliceLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder []string

	return &valueHolder
}

func (value stringSliceLabelValue) Value() interface{} {
	return value.value
}

func newStringSliceLabelValue(metadata StringSliceLabelValueMetadata, valueHolder interface{}) stringSliceLabelValue {
	value := *valueHolder.(*[]string)

	sort.Strings(value)

	sortedAndConstructedValue := strings.Join(value, ",")

	return stringSliceLabelValue{
		StringSliceLabelValueMetadata: metadata,
		value:                         sortedAndConstructedValue,
	}
}

func (metadata ByteSliceLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder []byte

	return &valueHolder
}

func (value byteSliceLabelValue) Value() interface{} {
	return value.value
}

func newByteSliceLabelValue(metadata ByteSliceLabelValueMetadata, valueHolder interface{}) byteSliceLabelValue {
	return byteSliceLabelValue{
		ByteSliceLabelValueMetadata: metadata,
		value:                       string(*valueHolder.(*[]byte)),
	}
}
