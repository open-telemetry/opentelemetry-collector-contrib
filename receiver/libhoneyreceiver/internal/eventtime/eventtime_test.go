// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetEventTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "empty string returns zero time",
			input:    "",
			expected: time.Time{}.UTC(),
		},
		{
			name:     "RFC3339Nano with Z timezone",
			input:    "2026-01-30T09:36:48Z",
			expected: time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC),
		},
		{
			name:     "RFC3339Nano with fractional seconds",
			input:    "2026-01-30T09:36:48.123456789Z",
			expected: time.Date(2026, 1, 30, 9, 36, 48, 123456789, time.UTC),
		},
		{
			name:     "RFC3339 with offset timezone",
			input:    "2026-01-30T04:36:48-05:00",
			expected: time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC),
		},
		{
			name:     "10-digit epoch seconds",
			input:    "1769767008",
			expected: time.Unix(1769767008, 0).UTC(),
		},
		{
			name:     "13-digit epoch milliseconds",
			input:    "1769767008000",
			expected: time.Unix(1769767008, 0).UTC(),
		},
		{
			name:     "13-digit epoch milliseconds with sub-second",
			input:    "1769767008500",
			expected: time.Unix(1769767008, 500000000).UTC(),
		},
		{
			name:     "16-digit epoch microseconds",
			input:    "1769767008000000",
			expected: time.Unix(1769767008, 0).UTC(),
		},
		{
			name:     "float epoch seconds",
			input:    "1769767008.5",
			expected: time.Unix(1769767008, 500000000).UTC(),
		},
		{
			name:     "unparseable string returns zero time",
			input:    "not-a-timestamp",
			expected: time.Time{}.UTC(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEventTime(tt.input)
			assert.Equal(t, tt.expected, result, "GetEventTime(%q)", tt.input)
		})
	}
}

func TestGetEventTime_DistinctSecondTimestamps(t *testing.T) {
	// This test validates that timestamps differing by 1 second
	// produce distinct results, matching the user scenario where
	// spans at T, T+1, T+3 should have different timestamps.
	formats := []struct {
		name   string
		timeT  string
		timeT1 string
		timeT3 string
	}{
		{
			name:   "RFC3339",
			timeT:  "2026-01-30T09:36:48Z",
			timeT1: "2026-01-30T09:36:49Z",
			timeT3: "2026-01-30T09:36:51Z",
		},
		{
			name:   "epoch seconds 10-digit",
			timeT:  "1769767008",
			timeT1: "1769767009",
			timeT3: "1769767011",
		},
		{
			name:   "epoch milliseconds 13-digit",
			timeT:  "1769767008000",
			timeT1: "1769767009000",
			timeT3: "1769767011000",
		},
	}

	for _, fmt := range formats {
		t.Run(fmt.name, func(t *testing.T) {
			tBase := GetEventTime(fmt.timeT)
			t1 := GetEventTime(fmt.timeT1)
			t3 := GetEventTime(fmt.timeT3)

			assert.False(t, tBase.IsZero(), "base time should not be zero")
			assert.False(t, t1.IsZero(), "T+1 time should not be zero")
			assert.False(t, t3.IsZero(), "T+3 time should not be zero")

			assert.NotEqual(t, tBase, t1, "T and T+1 should be different")
			assert.NotEqual(t, t1, t3, "T+1 and T+3 should be different")
			assert.NotEqual(t, tBase, t3, "T and T+3 should be different")

			// Verify the differences are the expected durations
			assert.Equal(t, time.Second, t1.Sub(tBase), "T+1 - T should be 1 second")
			assert.Equal(t, 3*time.Second, t3.Sub(tBase), "T+3 - T should be 3 seconds")
		})
	}
}

func TestGetEventTimeNano_PreservesNanosecondPrecision(t *testing.T) {
	// RFC3339Nano should preserve full nanosecond precision
	input := "2026-01-30T09:36:48.123456789Z"
	result := GetEventTimeNano(input)
	expected := time.Date(2026, 1, 30, 9, 36, 48, 123456789, time.UTC).UnixNano()
	assert.Equal(t, expected, result)
}

func TestGetEventTime_19DigitNanosecondEpoch(t *testing.T) {
	// 19-digit nanosecond epoch timestamps use float64 intermediate,
	// which can lose precision for nanosecond-level differences.
	// This test documents the precision limitation.
	t.Run("whole second nanosecond epoch", func(t *testing.T) {
		// "1769767008000000000" = 1769767008 seconds exactly
		result := GetEventTime("1769767008000000000")
		expected := time.Unix(1769767008, 0).UTC()
		assert.Equal(t, expected, result)
	})

	t.Run("distinct whole-second nanosecond epochs are distinguishable", func(t *testing.T) {
		t1 := GetEventTime("1769767008000000000")
		t2 := GetEventTime("1769767009000000000")
		assert.NotEqual(t, t1, t2, "timestamps 1 second apart should be different")
		assert.Equal(t, time.Second, t2.Sub(t1))
	})
}
