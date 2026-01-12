// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToFloat64Value(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected float64
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "float64",
			input:    NewFloat64Value(1.23),
			expected: 1.23,
		},
		{
			name:  "nil float64",
			input: NewNilFloat64Value(),
			isNil: true,
		},
		{
			name:     "string",
			input:    NewStringValue("1.23"),
			expected: 1.23,
		},
		{
			name:     "int64",
			input:    NewInt64Value(123),
			expected: 123.0,
		},
		{
			name:     "bool true",
			input:    NewBoolValue(true),
			expected: 1.0,
		},
		{
			name:     "bool false",
			input:    NewBoolValue(false),
			expected: 0.0,
		},
		{
			name:    "nil string",
			input:   NewNilStringValue(),
			wantErr: true,
		},
		{
			name:    "invalid string",
			input:   NewStringValue("not-a-float"),
			wantErr: true,
		},
		{
			name:    "nil int64",
			input:   NewNilInt64Value(),
			wantErr: true,
		},
		{
			name:    "nil bool",
			input:   NewNilBoolValue(),
			wantErr: true,
		},
		{
			name:     "pvalue double",
			input:    NewPValueValue(pcommon.NewValueDouble(10.5)),
			expected: 10.5,
		},
		{
			name:     "pvalue int",
			input:    NewPValueValue(pcommon.NewValueInt(100)),
			expected: 100.0,
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-float")),
			wantErr: true,
		},
		{
			name:     "pvalue bool true",
			input:    NewPValueValue(pcommon.NewValueBool(true)),
			expected: 1.0,
		},
		{
			name:     "pvalue bool false",
			input:    NewPValueValue(pcommon.NewValueBool(false)),
			expected: 0.0,
		},
		{
			name:    "nil pvalue",
			input:   NewNilPValueValue(),
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
			res, err := ToFloat64Value(tt.input)
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
