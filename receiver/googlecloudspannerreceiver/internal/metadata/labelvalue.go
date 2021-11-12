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

	"go.opentelemetry.io/collector/model/pdata"
)

type LabelValueMetadata interface {
	ValueMetadata
	NewLabelValue(value interface{}) LabelValue
}

type LabelValue interface {
	Metadata() LabelValueMetadata
	Value() interface{}
	SetValueTo(attributes pdata.AttributeMap)
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
	metadata StringLabelValueMetadata
	value    string
}

type int64LabelValue struct {
	metadata Int64LabelValueMetadata
	value    int64
}

type boolLabelValue struct {
	metadata BoolLabelValueMetadata
	value    bool
}

type stringSliceLabelValue struct {
	metadata StringSliceLabelValueMetadata
	value    string
}

type byteSliceLabelValue struct {
	metadata ByteSliceLabelValueMetadata
	value    string
}

func (m queryLabelValueMetadata) Name() string {
	return m.name
}

func (m queryLabelValueMetadata) ColumnName() string {
	return m.columnName
}

func (m StringLabelValueMetadata) NewLabelValue(value interface{}) LabelValue {
	return newStringLabelValue(m, value)
}

func (m StringLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder string

	return &valueHolder
}

func (v stringLabelValue) Metadata() LabelValueMetadata {
	return v.metadata
}

func (v stringLabelValue) Value() interface{} {
	return v.value
}

func (v stringLabelValue) SetValueTo(attributes pdata.AttributeMap) {
	attributes.InsertString(v.metadata.name, v.value)
}

func newStringLabelValue(metadata StringLabelValueMetadata, valueHolder interface{}) stringLabelValue {
	return stringLabelValue{
		metadata: metadata,
		value:    *valueHolder.(*string),
	}
}

func (m Int64LabelValueMetadata) NewLabelValue(value interface{}) LabelValue {
	return newInt64LabelValue(m, value)
}

func (m Int64LabelValueMetadata) ValueHolder() interface{} {
	var valueHolder int64

	return &valueHolder
}

func (v int64LabelValue) Metadata() LabelValueMetadata {
	return v.metadata
}

func (v int64LabelValue) Value() interface{} {
	return v.value
}

func (v int64LabelValue) SetValueTo(attributes pdata.AttributeMap) {
	attributes.InsertInt(v.metadata.name, v.value)
}

func newInt64LabelValue(metadata Int64LabelValueMetadata, valueHolder interface{}) int64LabelValue {
	return int64LabelValue{
		metadata: metadata,
		value:    *valueHolder.(*int64),
	}
}

func (m BoolLabelValueMetadata) NewLabelValue(value interface{}) LabelValue {
	return newBoolLabelValue(m, value)
}

func (m BoolLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder bool

	return &valueHolder
}

func (v boolLabelValue) Metadata() LabelValueMetadata {
	return v.metadata
}

func (v boolLabelValue) Value() interface{} {
	return v.value
}

func (v boolLabelValue) SetValueTo(attributes pdata.AttributeMap) {
	attributes.InsertBool(v.metadata.name, v.value)
}

func newBoolLabelValue(metadata BoolLabelValueMetadata, valueHolder interface{}) boolLabelValue {
	return boolLabelValue{
		metadata: metadata,
		value:    *valueHolder.(*bool),
	}
}

func (m StringSliceLabelValueMetadata) NewLabelValue(value interface{}) LabelValue {
	return newStringSliceLabelValue(m, value)
}

func (m StringSliceLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder []string

	return &valueHolder
}

func (v stringSliceLabelValue) Metadata() LabelValueMetadata {
	return v.metadata
}

func (v stringSliceLabelValue) Value() interface{} {
	return v.value
}

func (v stringSliceLabelValue) SetValueTo(attributes pdata.AttributeMap) {
	attributes.InsertString(v.metadata.name, v.value)
}

func newStringSliceLabelValue(metadata StringSliceLabelValueMetadata, valueHolder interface{}) stringSliceLabelValue {
	value := *valueHolder.(*[]string)

	sort.Strings(value)

	sortedAndConstructedValue := strings.Join(value, ",")

	return stringSliceLabelValue{
		metadata: metadata,
		value:    sortedAndConstructedValue,
	}
}

func (m ByteSliceLabelValueMetadata) NewLabelValue(value interface{}) LabelValue {
	return newByteSliceLabelValue(m, value)
}

func (m ByteSliceLabelValueMetadata) ValueHolder() interface{} {
	var valueHolder []byte

	return &valueHolder
}

func (v byteSliceLabelValue) Metadata() LabelValueMetadata {
	return v.metadata
}

func (v byteSliceLabelValue) Value() interface{} {
	return v.value
}

func (v byteSliceLabelValue) SetValueTo(attributes pdata.AttributeMap) {
	attributes.InsertString(v.metadata.name, v.value)
}

func newByteSliceLabelValue(metadata ByteSliceLabelValueMetadata, valueHolder interface{}) byteSliceLabelValue {
	return byteSliceLabelValue{
		metadata: metadata,
		value:    string(*valueHolder.(*[]byte)),
	}
}
