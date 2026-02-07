// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected bool
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "bool true",
			input:    NewBoolValue(true),
			expected: true,
		},
		{
			name:     "bool false",
			input:    NewBoolValue(false),
			expected: false,
		},
		{
			name:  "nil bool",
			input: NewNilBoolValue(),
			isNil: true,
		},
		{
			name:     "string true",
			input:    NewStringValue("true"),
			expected: true,
		},
		{
			name:     "string false",
			input:    NewStringValue("false"),
			expected: false,
		},
		{
			name:    "invalid string",
			input:   NewStringValue("not-a-bool"),
			wantErr: true,
		},
		{
			name:     "int64 non-zero",
			input:    NewInt64Value(1),
			expected: true,
		},
		{
			name:     "int64 zero",
			input:    NewInt64Value(0),
			expected: false,
		},
		{
			name:     "float64 non-zero",
			input:    NewFloat64Value(1.5),
			expected: true,
		},
		{
			name:     "float64 zero",
			input:    NewFloat64Value(0.0),
			expected: false,
		},
		{
			name:     "pvalue int non-zero",
			input:    NewPValueValue(pcommon.NewValueInt(10)),
			expected: true,
		},
		{
			name:     "pvalue int zero",
			input:    NewPValueValue(pcommon.NewValueInt(0)),
			expected: false,
		},
		{
			name:     "pvalue double non-zero",
			input:    NewPValueValue(pcommon.NewValueDouble(1.2)),
			expected: true,
		},
		{
			name:     "pvalue double zero",
			input:    NewPValueValue(pcommon.NewValueDouble(0.0)),
			expected: false,
		},
		{
			name:     "pvalue string true",
			input:    NewPValueValue(pcommon.NewValueStr("true")),
			expected: true,
		},
		{
			name:     "pvalue string false",
			input:    NewPValueValue(pcommon.NewValueStr("false")),
			expected: false,
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-bool")),
			wantErr: true,
		},
		{
			name:     "pvalue bool true",
			input:    NewPValueValue(pcommon.NewValueBool(true)),
			expected: true,
		},
		{
			name:     "pvalue bool false",
			input:    NewPValueValue(pcommon.NewValueBool(false)),
			expected: false,
		},
		{
			name:    "pvalue empty",
			input:   NewPValueValue(pcommon.NewValueEmpty()),
			wantErr: true,
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
			res, err := ToBoolValue(tt.input)
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
