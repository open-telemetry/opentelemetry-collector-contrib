// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToPValueValue(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected pcommon.Value
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "pvalue",
			input:    NewPValueValue(pcommon.NewValueInt(42)),
			expected: pcommon.NewValueInt(42),
		},
		{
			name:  "nil pvalue",
			input: NewNilPValueValue(),
			isNil: true,
		},
		{
			name:     "string",
			input:    NewStringValue("foo"),
			expected: pcommon.NewValueStr("foo"),
		},
		{
			name:     "int64",
			input:    NewInt64Value(123),
			expected: pcommon.NewValueInt(123),
		},
		{
			name:     "float64",
			input:    NewFloat64Value(1.23),
			expected: pcommon.NewValueDouble(1.23),
		},
		{
			name:     "bool true",
			input:    NewBoolValue(true),
			expected: pcommon.NewValueBool(true),
		},
		{
			name:    "nil string",
			input:   NewNilStringValue(),
			wantErr: true,
		},
		{
			name:    "nil int64",
			input:   NewNilInt64Value(),
			wantErr: true,
		},
		{
			name:    "nil float64",
			input:   NewNilFloat64Value(),
			wantErr: true,
		},
		{
			name:    "nil bool",
			input:   NewNilBoolValue(),
			wantErr: true,
		},
		{
			name:  "pmap",
			input: NewPMapValue(pcommon.NewMap()),
			// Note: ToPValueValue creates a new ValueMap from the Map
			expected: pcommon.NewValueMap(),
		},
		{
			name:    "nil pmap",
			input:   NewNilPMapValue(),
			wantErr: true,
		},
		{
			name:  "pslice",
			input: NewPSliceValue(pcommon.NewSlice()),
			// Note: ToPValueValue creates a new ValueSlice from the Slice
			expected: pcommon.NewValueSlice(),
		},
		{
			name:    "nil pslice",
			input:   NewNilPSliceValue(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToPValueValue(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.isNil, res.IsNil())
			if !tt.isNil {
				assert.Equal(t, tt.expected, res.Val())
			}
		})
	}
}
