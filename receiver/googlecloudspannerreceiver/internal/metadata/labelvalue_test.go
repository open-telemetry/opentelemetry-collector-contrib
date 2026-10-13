// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestLockRequestJSONLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestJSONValueType)

	assert.Equal(t, LockRequestJSONValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *string

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

	labelValue.ModifyValue(labelName)
	assert.Equal(t, labelName, labelValue.Value())
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

func TestLockRequestJSONLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestJSONValueType)
	labelValue := lockRequestJSONLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, LockRequestJSONValueType, labelValue.Metadata().ValueType())
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

func TestNewLockRequestJSONLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, LockRequestJSONValueType)

	testCases := []struct {
		name          string
		inputJSON     string
		expectedValue string
	}{
		{
			name:          "With transaction_tag",
			inputJSON:     `[{"lock_mode":"lockMode1","column":"column1","transaction_tag":"tag1"}]`,
			expectedValue: "{lockMode1,column1,tag1}",
		},
		{
			name:          "Without transaction_tag (empty string and omitted)",
			inputJSON:     `[{"lock_mode":"lockMode1","column":"column1","transaction_tag":""},{"lock_mode":"lockMode2","column":"column2"}]`,
			expectedValue: "{lockMode1,column1},{lockMode2,column2}",
		},
		{
			name:          "Null transaction_tag",
			inputJSON:     `[{"lock_mode":"lockMode1","column":"column1","transaction_tag":null}]`,
			expectedValue: "{lockMode1,column1}",
		},
		{
			name: "Duplicates and unsorted order",
			inputJSON: `[
				{"lock_mode":"lockMode2","column":"column2","transaction_tag":"tag2"},
				{"lock_mode":"lockMode1","column":"column1","transaction_tag":"tag1"},
				{"lock_mode":"lockMode1","column":"column1","transaction_tag":"tag1"},
				{"lock_mode":"lockMode2","column":"column2","transaction_tag":"tag2"},
				{"lock_mode":"lockMode3","column":"column3","transaction_tag":""}
			]`,
			expectedValue: "{lockMode1,column1,tag1},{lockMode2,column2,tag2},{lockMode3,column3}",
		},
		{
			name:          "Key-value transaction_tag",
			inputJSON:     `[{"lock_mode":"ReaderShared","column":"Singers.SingerInfo","transaction_tag":"app=concert,env=dev"}]`,
			expectedValue: "{ReaderShared,Singers.SingerInfo,app=concert,env=dev}",
		},
		{
			name:          "Empty JSON array",
			inputJSON:     "[]",
			expectedValue: "",
		},
		{
			name:          "Empty JSON object",
			inputJSON:     "{}",
			expectedValue: "",
		},
		{
			name:          "Null JSON string",
			inputJSON:     "null",
			expectedValue: "",
		},
		{
			name:          "Empty string",
			inputJSON:     "",
			expectedValue: "",
		},
		{
			name:          "Invalid JSON",
			inputJSON:     "invalid-json",
			expectedValue: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.inputJSON
			valueHolder := &value

			labelValue := newLockRequestJSONLabelValue(metadata, valueHolder)

			assert.Equal(t, LockRequestJSONValueType, labelValue.Metadata().ValueType())
			assert.Equal(t, tc.expectedValue, labelValue.Value())
		})
	}
}
