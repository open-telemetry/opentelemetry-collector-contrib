// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToInt64Value(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected int64
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "int64",
			input:    NewInt64Value(42),
			expected: 42,
		},
		{
			name:  "nil int64",
			input: NewNilInt64Value(),
			isNil: true,
		},
		{
			name:     "string",
			input:    NewStringValue("123"),
			expected: 123,
		},
		{
			name:     "float64",
			input:    NewFloat64Value(1.9),
			expected: 1,
		},
		{
			name:     "bool true",
			input:    NewBoolValue(true),
			expected: 1,
		},
		{
			name:     "bool false",
			input:    NewBoolValue(false),
			expected: 0,
		},
		{
			name:     "pvalue string",
			input:    NewPValueValue(pcommon.NewValueStr("100")),
			expected: 100,
		},
		{
			name:     "pvalue int",
			input:    NewPValueValue(pcommon.NewValueInt(100)),
			expected: 100,
		},
		{
			name:     "pvalue float64",
			input:    NewPValueValue(pcommon.NewValueDouble(100.9)),
			expected: 100,
		},
		{
			name:     "pvalue bool true",
			input:    NewPValueValue(pcommon.NewValueBool(true)),
			expected: 1,
		},
		{
			name:     "pvalue bool false",
			input:    NewPValueValue(pcommon.NewValueBool(false)),
			expected: 0,
		},
		{
			name:    "nil string",
			input:   NewNilStringValue(),
			wantErr: true,
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-an-int")),
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
			name:    "unsupported type (PMap)",
			input:   NewPMapValue(pcommon.NewMap()),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToInt64Value(tt.input)
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
