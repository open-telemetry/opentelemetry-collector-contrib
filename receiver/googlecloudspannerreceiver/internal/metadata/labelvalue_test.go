// Copyright The OpenTelemetry Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestStringLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringValueType)

	assert.Equal(t, StringValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *string

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestInt64LabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntValueType)

	assert.Equal(t, IntValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *int64

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestBoolLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, BoolValueType)

	assert.Equal(t, BoolValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *bool

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestStringSliceLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringSliceValueType)

	assert.Equal(t, StringSliceValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]string

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestByteSliceLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, ByteSliceValueType)

	assert.Equal(t, ByteSliceValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]byte

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestLockRequestSliceLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestSliceValueType)

	assert.Equal(t, LockRequestSliceValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]*lockRequest

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestUnknownLabelValueMetadata(t *testing.T) {
	metadata, err := NewLabelValueMetadata(labelName, labelColumnName, UnknownValueType)

	require.Error(t, err)
	require.Nil(t, metadata)
}

func TestStringLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringValueType)
	labelValue := stringLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, StringValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.Str())
}

func TestInt64LabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntValueType)
	labelValue := int64LabelValue{
		metadata: metadata,
		value:    int64Value,
	}

	assert.Equal(t, IntValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, int64Value, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, int64Value, attributeValue.Int())
}

func TestBoolLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, BoolValueType)
	labelValue := boolLabelValue{
		metadata: metadata,
		value:    boolValue,
	}

	assert.Equal(t, BoolValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, boolValue, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, boolValue, attributeValue.Bool())
}

func TestStringSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringSliceValueType)
	labelValue := stringSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, StringSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.Str())
}

func TestByteSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, ByteSliceValueType)
	labelValue := byteSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, ByteSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.Str())
}

func TestLockRequestSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestSliceValueType)
	labelValue := lockRequestSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, LockRequestSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pcommon.NewMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.Str())
}

func TestNewStringLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringValueType)
	value := stringValue
	valueHolder := &value

	labelValue := newStringLabelValue(metadata, valueHolder)

	assert.Equal(t, StringValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())
}

func TestNewInt64LabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntValueType)
	value := int64Value
	valueHolder := &value

	labelValue := newInt64LabelValue(metadata, valueHolder)

	assert.Equal(t, IntValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, int64Value, labelValue.Value())
}

func TestNewBoolLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, BoolValueType)
	value := boolValue
	valueHolder := &value

	labelValue := newBoolLabelValue(metadata, valueHolder)

	assert.Equal(t, BoolValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, boolValue, labelValue.Value())
}

func TestNewStringSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringSliceValueType)
	value := []string{"b", "a", "c"}
	expectedValue := "a,b,c"
	valueHolder := &value

	labelValue := newStringSliceLabelValue(metadata, valueHolder)

	assert.Equal(t, StringSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, expectedValue, labelValue.Value())
}

func TestNewByteSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, ByteSliceValueType)
	value := []byte(stringValue)
	valueHolder := &value

	labelValue := newByteSliceLabelValue(metadata, valueHolder)

	assert.Equal(t, ByteSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, stringValue, labelValue.Value())
}

func TestNewLockRequestSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestSliceValueType)
	value := []*lockRequest{
		{LockMode: "lockMode1", Column: "column1", TransactionTag: "tag1"},
		{LockMode: "lockMode2", Column: "column2", TransactionTag: "tag2"},
		{LockMode: "lockMode1", Column: "column1", TransactionTag: "tag1"},
		{LockMode: "lockMode2", Column: "column2", TransactionTag: "tag2"},
		{LockMode: "lockMode3", Column: "column3"},
	}
	expectedValue := "{lockMode1,column1,tag1},{lockMode2,column2,tag2},{lockMode3,column3}"
	valueHolder := &value

	labelValue := newLockRequestSliceLabelValue(metadata, valueHolder)

	assert.Equal(t, LockRequestSliceValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, expectedValue, labelValue.Value())
}
