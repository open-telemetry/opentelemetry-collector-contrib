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

func TestToTimeValue(t *testing.T) {
	now := time.Now().Truncate(time.Second) // RFC3339 might lose precision depending on implementation
	nowRFC := now.Format(time.RFC3339)
	parsedNow, _ := time.Parse(time.RFC3339, nowRFC)

	tests := []struct {
		name     string
		input    Value
		expected time.Time
		isNil    bool
		wantErr  bool
	}{
		{
			name:     "time",
			input:    NewTimeValue(parsedNow),
			expected: parsedNow,
		},
		{
			name:  "nil time",
			input: NewNilTimeValue(),
			isNil: true,
		},
		{
			name:     "string RFC3339",
			input:    NewStringValue(nowRFC),
			expected: parsedNow,
		},
		{
			name:    "invalid string",
			input:   NewStringValue("not-a-time"),
			wantErr: true,
		},
		{
			name:    "nil string RFC3339",
			input:   NewNilStringValue(),
			wantErr: true,
		},
		{
			name:     "int64 (nanos)",
			input:    NewInt64Value(parsedNow.UnixNano()),
			expected: parsedNow,
		},
		{
			name:    "nil int64",
			input:   NewNilInt64Value(),
			wantErr: true,
		},
		{
			name:     "pvalue string RFC3339",
			input:    NewPValueValue(pcommon.NewValueStr(nowRFC)),
			expected: parsedNow,
		},
		{
			name:    "pvalue invalid string",
			input:   NewPValueValue(pcommon.NewValueStr("not-a-time")),
			wantErr: true,
		},
		{
			name:     "pvalue int (nanos)",
			input:    NewPValueValue(pcommon.NewValueInt(parsedNow.UnixNano())),
			expected: parsedNow,
		},
		{
			name:    "nil pvalue",
			input:   NewNilPValueValue(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ToTimeValue(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.isNil, res.IsNil())
			if !tt.isNil {
				assert.True(t, tt.expected.Equal(res.Val()))
			}
		})
	}
}
