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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestStringLabelValueMetadata(t *testing.T) {
	metadata := StringLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *string

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestInt64LabelValueMetadata(t *testing.T) {
	metadata := Int64LabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *int64

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestBoolLabelValueMetadata(t *testing.T) {
	metadata := BoolLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *bool

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestStringSliceLabelValueMetadata(t *testing.T) {
	metadata := StringSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]string

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestByteSliceLabelValueMetadata(t *testing.T) {
	metadata := ByteSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	assert.Equal(t, labelName, metadata.Name())
	assert.Equal(t, labelColumnName, metadata.ColumnName())

	var expectedType *[]byte

	assert.IsType(t, expectedType, metadata.ValueHolder())
}

func TestStringLabelValue(t *testing.T) {
	metadata := StringLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue := stringLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pdata.NewAttributeMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.StringVal())
}

func TestInt64LabelValue(t *testing.T) {
	metadata := Int64LabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue := int64LabelValue{
		metadata: metadata,
		value:    int64Value,
	}

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, int64Value, labelValue.Value())

	attributes := pdata.NewAttributeMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, int64Value, attributeValue.IntVal())
}

func TestBoolLabelValue(t *testing.T) {
	metadata := BoolLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue := boolLabelValue{
		metadata: metadata,
		value:    boolValue,
	}

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, boolValue, labelValue.Value())

	attributes := pdata.NewAttributeMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, boolValue, attributeValue.BoolVal())
}

func TestStringSliceLabelValue(t *testing.T) {
	metadata := StringSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue := stringSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pdata.NewAttributeMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.StringVal())
}

func TestByteSliceLabelValue(t *testing.T) {
	metadata := ByteSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue := byteSliceLabelValue{
		metadata: metadata,
		value:    stringValue,
	}

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, stringValue, labelValue.Value())

	attributes := pdata.NewAttributeMap()

	labelValue.SetValueTo(attributes)

	attributeValue, exists := attributes.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, stringValue, attributeValue.StringVal())
}

func TestNewQueryLabelValueMetadata(t *testing.T) {
	metadata := newQueryLabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewStringLabelValueMetadata(t *testing.T) {
	metadata := NewStringLabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewInt64LabelValueMetadata(t *testing.T) {
	metadata := NewInt64LabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewBoolLabelValueMetadata(t *testing.T) {
	metadata := NewBoolLabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewStringSliceLabelValueMetadata(t *testing.T) {
	metadata := NewStringSliceLabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewByteSliceLabelValueMetadata(t *testing.T) {
	metadata := NewByteSliceLabelValueMetadata(labelName, labelColumnName)

	assert.Equal(t, labelName, metadata.name)
	assert.Equal(t, labelColumnName, metadata.columnName)
}

func TestNewStringLabelValue(t *testing.T) {
	metadata := StringLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	value := stringValue
	valueHolder := &value

	labelValue := newStringLabelValue(metadata, valueHolder)

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, stringValue, labelValue.Value())
}

func TestNewInt64LabelValue(t *testing.T) {
	metadata := Int64LabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	value := int64Value
	valueHolder := &value

	labelValue := newInt64LabelValue(metadata, valueHolder)

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, int64Value, labelValue.Value())
}

func TestNewBoolLabelValue(t *testing.T) {
	metadata := BoolLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	value := boolValue
	valueHolder := &value

	labelValue := newBoolLabelValue(metadata, valueHolder)

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, boolValue, labelValue.Value())
}

func TestNewStringSliceLabelValue(t *testing.T) {
	metadata := StringSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	value := []string{"b", "a", "c"}
	expectedValue := "a,b,c"
	valueHolder := &value

	labelValue := newStringSliceLabelValue(metadata, valueHolder)

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, expectedValue, labelValue.Value())
}

func TestNewByteSliceLabelValue(t *testing.T) {
	metadata := ByteSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	value := []byte(stringValue)
	valueHolder := &value

	labelValue := newByteSliceLabelValue(metadata, valueHolder)

	assert.Equal(t, metadata, labelValue.Metadata())
	assert.Equal(t, stringValue, labelValue.Value())
}
