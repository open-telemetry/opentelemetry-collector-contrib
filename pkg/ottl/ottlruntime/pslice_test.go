// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToPSliceValue(t *testing.T) {
	s := pcommon.NewSlice()
	s.AppendEmpty().SetStr("foo")

	tests := []struct {
		name     string
		input    Value
		expected pcommon.Slice
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "pslice",
			input:    NewPSliceValue(s),
			expected: s,
		},
		{
			name:  "nil pslice",
			input: NewNilPSliceValue(),
			isNil: true,
		},
		{
			name:     "pvalue slice",
			input:    NewPValueValue(pcommon.NewValueSlice()),
			expected: pcommon.NewSlice(),
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-slice")),
			wantErr: true,
		},
		{
			name:    "nil pvalue",
			input:   NewNilPValueValue(),
			wantErr: true,
		},
		{
			name:    "unsupported type (string)",
			input:   NewStringValue("foo"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToPSliceValue(tt.input)
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
