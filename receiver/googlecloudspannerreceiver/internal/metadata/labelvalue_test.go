// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"math"
	"testing"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/spanner"
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

func TestIntSliceLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntSliceValueType)

	assert.Equal(t, IntSliceValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]int64

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

func TestIntSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntSliceValueType)
	labelValue := intSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, IntSliceValueType, labelValue.Metadata().ValueType())
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

func TestNewIntSliceLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, IntSliceValueType)

	tests := []struct {
		name          string
		value         []int64
		expectedValue string
	}{
		{
			name:          "Sorting check",
			value:         []int64{2, 10, 1, 3},
			expectedValue: "1,2,3,10",
		},
		{
			name:          "With single value",
			value:         []int64{42},
			expectedValue: "42",
		},
		{
			name:          "With empty array",
			value:         []int64{},
			expectedValue: "",
		},
		{
			name:          "NULL array",
			value:         nil,
			expectedValue: "",
		},
		{
			name:          "Duplicates",
			value:         []int64{2, 1, 2, 3, 1},
			expectedValue: "1,2,3",
		},
		{
			name:          "Many duplicates and out of order",
			value:         []int64{8, 3, 5, 7, 4, 4, 3, 5, 10, 1},
			expectedValue: "1,3,4,5,7,8,10",
		},
		{
			name:          "Full int64 values",
			value:         []int64{math.MaxInt64, 0, math.MinInt64},
			expectedValue: "-9223372036854775808,0,9223372036854775807",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valueHolder := &tt.value
			labelValue := newIntSliceLabelValue(metadata, valueHolder)

			assert.Equal(t, IntSliceValueType, labelValue.Metadata().ValueType())
			assert.Equal(t, tt.expectedValue, labelValue.Value())
		})
	}
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

func TestDateLabelValueMetadata(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, DateValueType)

	assert.Equal(t, DateValueType, metadata.ValueType())
	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *spanner.NullDate

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestDateLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, DateValueType)
	labelValue := dateLabelValue{
		metadata: metadata,
		value:    "2023-10-01",
	}

	assert.Equal(t, DateValueType, labelValue.Metadata().ValueType())
	assert.Equal(t, "2023-10-01", labelValue.Value())

	attributes := pcommon.NewMap()
	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, "2023-10-01", attributeValue.Str())

	labelValue.ModifyValue("2023-10-02")
	assert.Equal(t, "2023-10-02", labelValue.Value())
}

func TestNewDateLabelValue(t *testing.T) {
	metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, DateValueType)

	tests := []struct {
		name          string
		value         spanner.NullDate
		expectedValue string
	}{
		{
			name: "Valid standard date 2026-09-10",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2026, Month: 9, Day: 10},
				Valid: true,
			},
			expectedValue: "2026-09-10",
		},
		{
			name: "Valid standard date 2023-10-01",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2023, Month: 10, Day: 1},
				Valid: true,
			},
			expectedValue: "2023-10-01",
		},
		{
			name: "Valid leap year date 2024-02-29",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2024, Month: 2, Day: 29},
				Valid: true,
			},
			expectedValue: "2024-02-29",
		},
		{
			name: "Valid century leap year date 2000-02-29",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2000, Month: 2, Day: 29},
				Valid: true,
			},
			expectedValue: "2000-02-29",
		},
		{
			name: "Valid minimum boundary date 0001-01-01",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 1, Month: 1, Day: 1},
				Valid: true,
			},
			expectedValue: "0001-01-01",
		},
		{
			name: "Valid maximum boundary date 9999-12-31",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 9999, Month: 12, Day: 31},
				Valid: true,
			},
			expectedValue: "9999-12-31",
		},
		{
			name: "Valid Unix epoch date 1970-01-01",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 1970, Month: 1, Day: 1},
				Valid: true,
			},
			expectedValue: "1970-01-01",
		},
		{
			name: "Valid zero-padded single-digit month and day 2026-01-05",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2026, Month: 1, Day: 5},
				Valid: true,
			},
			expectedValue: "2026-01-05",
		},
		{
			name: "Valid zero-padded year under 1000 (0099-03-07)",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 99, Month: 3, Day: 7},
				Valid: true,
			},
			expectedValue: "0099-03-07",
		},
		{
			name:          "Null date (Valid=false)",
			value:         spanner.NullDate{Valid: false},
			expectedValue: "",
		},
		{
			name: "Null date with non-zero Date struct (Valid=false)",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2026, Month: 9, Day: 10},
				Valid: false,
			},
			expectedValue: "",
		},
		{
			name: "Empty zero civil.Date with Valid=true",
			value: spanner.NullDate{
				Date:  civil.Date{},
				Valid: true,
			},
			expectedValue: "",
		},
		{
			name: "Invalid calendar date (non-leap year Feb 29)",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2023, Month: 2, Day: 29},
				Valid: true,
			},
			expectedValue: "",
		},
		{
			name: "Invalid calendar date (month out of range)",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2023, Month: 13, Day: 1},
				Valid: true,
			},
			expectedValue: "",
		},
		{
			name: "Invalid calendar date (day out of range)",
			value: spanner.NullDate{
				Date:  civil.Date{Year: 2023, Month: 4, Day: 31},
				Valid: true,
			},
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valueHolder := &tt.value
			labelValue := newDateLabelValue(metadata, valueHolder)

			assert.Equal(t, DateValueType, labelValue.Metadata().ValueType())
			assert.Equal(t, tt.expectedValue, labelValue.Value())
		})
	}
}
