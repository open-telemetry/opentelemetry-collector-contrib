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

	labelValue :=
		stringLabelValue{
			StringLabelValueMetadata: metadata,
			value:                    stringValue,
		}

	assert.Equal(t, metadata, labelValue.StringLabelValueMetadata)
	assert.Equal(t, stringValue, labelValue.Value())
}

func TestInt64LabelValue(t *testing.T) {
	metadata := Int64LabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue :=
		int64LabelValue{
			Int64LabelValueMetadata: metadata,
			value:                   int64Value,
		}

	assert.Equal(t, metadata, labelValue.Int64LabelValueMetadata)
	assert.Equal(t, int64Value, labelValue.Value())
}

func TestBoolLabelValue(t *testing.T) {
	metadata := BoolLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue :=
		boolLabelValue{
			BoolLabelValueMetadata: metadata,
			value:                  boolValue,
		}

	assert.Equal(t, metadata, labelValue.BoolLabelValueMetadata)
	assert.Equal(t, boolValue, labelValue.Value())
}

func TestStringSliceLabelValue(t *testing.T) {
	metadata := StringSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue :=
		stringSliceLabelValue{
			StringSliceLabelValueMetadata: metadata,
			value:                         stringValue,
		}

	assert.Equal(t, metadata, labelValue.StringSliceLabelValueMetadata)
	assert.Equal(t, stringValue, labelValue.Value())
}

func TestByteSliceLabelValue(t *testing.T) {
	metadata := ByteSliceLabelValueMetadata{
		queryLabelValueMetadata{
			name:       labelName,
			columnName: labelColumnName,
		},
	}

	labelValue :=
		byteSliceLabelValue{
			ByteSliceLabelValueMetadata: metadata,
			value:                       stringValue,
		}

	assert.Equal(t, metadata, labelValue.ByteSliceLabelValueMetadata)
	assert.Equal(t, stringValue, labelValue.Value())
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

	assert.Equal(t, metadata, labelValue.StringLabelValueMetadata)
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

	assert.Equal(t, metadata, labelValue.Int64LabelValueMetadata)
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

	assert.Equal(t, metadata, labelValue.BoolLabelValueMetadata)
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

	assert.Equal(t, metadata, labelValue.StringSliceLabelValueMetadata)
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

	assert.Equal(t, metadata, labelValue.ByteSliceLabelValueMetadata)
	assert.Equal(t, stringValue, labelValue.Value())
}
