// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected string
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "string",
			input:    NewStringValue("foo"),
			expected: "foo",
		},
		{
			name:  "nil string",
			input: NewNilStringValue(),
			isNil: true,
		},
		{
			name:     "int64",
			input:    NewInt64Value(123),
			expected: "123",
		},
		{
			name:     "float64",
			input:    NewFloat64Value(1.23),
			expected: "1.23",
		},
		{
			name:     "bool true",
			input:    NewBoolValue(true),
			expected: "true",
		},
		{
			name:     "pvalue string",
			input:    NewPValueValue(pcommon.NewValueStr("bar")),
			expected: "bar",
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
			name:    "nil pmap",
			input:   NewNilPMapValue(),
			wantErr: true,
		},
		{
			name:    "nil pslice",
			input:   NewNilPSliceValue(),
			wantErr: true,
		},
		{
			name:    "nil pvalue",
			input:   NewNilPValueValue(),
			wantErr: true,
		},
		{
			name:     "pmap",
			input:    NewPMapValue(pcommon.NewMap()),
			expected: "{}",
		},
		{
			name:     "pslice",
			input:    NewPSliceValue(pcommon.NewSlice()),
			expected: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToStringValue(tt.input)
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
