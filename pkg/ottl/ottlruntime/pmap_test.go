// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToPMapValue(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("foo", "bar")

	tests := []struct {
		name     string
		input    Value
		expected pcommon.Map
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "pmap",
			input:    NewPMapValue(m),
			expected: m,
		},
		{
			name:  "nil pmap",
			input: NewNilPMapValue(),
			isNil: true,
		},
		{
			name:     "pvalue map",
			input:    NewPValueValue(pcommon.NewValueMap()),
			expected: pcommon.NewMap(),
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-map")),
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
			res, err := ToPMapValue(tt.input)
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
