// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestToDurationValue(t *testing.T) {
	d := 5 * time.Second

	tests := []struct {
		name     string
		input    Value
		expected time.Duration
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "duration",
			input:    NewDurationValue(d),
			expected: d,
		},
		{
			name:  "nil duration",
			input: NewNilDurationValue(),
			isNil: true,
		},
		{
			name:     "string duration",
			input:    NewStringValue("5s"),
			expected: d,
		},
		{
			name:    "invalid string",
			input:   NewStringValue("not-a-duration"),
			wantErr: true,
		},
		{
			name:    "nil string",
			input:   NewNilStringValue(),
			wantErr: true,
		},
		{
			name:     "int64 (nanos)",
			input:    NewInt64Value(d.Nanoseconds()),
			expected: d,
		},
		{
			name:    "nil int64",
			input:   NewNilInt64Value(),
			wantErr: true,
		},
		{
			name:     "pvalue int (nanos)",
			input:    NewPValueValue(pcommon.NewValueInt(d.Nanoseconds())),
			expected: d,
		},
		{
			name:     "pvalue string",
			input:    NewPValueValue(pcommon.NewValueStr("5s")),
			expected: d,
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-duration")),
			wantErr: true,
		},
		{
			name:    "nil pvalue",
			input:   NewNilPValueValue(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToDurationValue(tt.input)
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
