// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Keep the original license.

// Copyright 2019 Dmitry A. Mottl. All rights reserved.
// Use of this source code is governed by MIT license
// that can be found in the LICENSE file.

package ctimefmt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	format1 = "%Y-%m-%d %H:%M:%S.%f"
	format2 = "%Y-%m-%d %l:%M:%S.%L %P, %a"
	format3 = "%Y-%m-%d %T.%s +0000"
	value1  = "2019-01-02 15:04:05.666666"
	value2  = "2019-01-02 3:04:05.666 pm, Wed"
	value3  = "2025-07-07 01:07:27.123456789 +0000"
	dt1     = time.Date(2019, 1, 2, 15, 4, 5, 666666000, time.UTC)
	dt2     = time.Date(2019, 1, 2, 15, 4, 5, 666000000, time.UTC)
	dt3     = time.Date(2025, 7, 7, 1, 7, 27, 123456789, time.UTC)
	layout1 = "2006-1-2 15:4:5.999999"
	layout2 = "2006-1-2 3:4:5.999 pm, Mon"
)

func TestFormat(t *testing.T) {
	s, err := Format(format1, dt1)
	require.NoError(t, err)
	assert.Equal(t, s, value1, "Given: %v, expected: %v", s, value1)

	s, err = Format(format2, dt1)
	require.NoError(t, err)
	assert.Equal(t, s, value2, "Given: %v, expected: %v", s, value2)

	s, err = Format(format3, dt3)
	require.NoError(t, err)
	assert.Equal(t, s, value3, "Given: %v, expected: %v", s, value2)
}

func TestParse(t *testing.T) {
	dt, layout, err := Parse(format1, func(native string) (time.Time, error) { return time.Parse(native, value1) })
	require.NoError(t, err)
	assert.Equal(t, dt1, dt, "Given: %v, expected: %v", dt, dt1)
	assert.Equal(t, layout1, layout)

	dt, layout, err = Parse(format2, func(native string) (time.Time, error) { return time.Parse(native, value2) })
	require.NoError(t, err)
	assert.Equal(t, dt2, dt, "Given: %v, expected: %v", dt, dt2)
	assert.Equal(t, layout2, layout)
}

func TestFlexibleParse(t *testing.T) {
	// These test cases cover only the format specifiers that are unique to OTel or have conflicting behavior vs strptime.
	// See ../../strptime_test.go for test cases that verify OTel's behavior matches strptime.
	want := time.Date(2019, 1, 2, 15, 4, 5, 0, time.UTC)
	for _, tc := range []struct {
		name, format, input string
	}{
		{"baseline", "%Y-%m-%dT%H:%M:%S%z", "2019-1-2T15:4:5Z"},
		// Bespoke to OTel
		{"o", "%Y-%o-%dT%H:%M:%S%z", "2019-1-2T15:4:5Z"},
		{"q", "%Y-%q-%dT%H:%M:%S%z", "2019-1-2T15:4:5Z"},
		// Conflicting with normal strptime behavior
		{"X", "%Y-%m-%dT%X%z", "2019-1-2T15:4:5Z"},
		{"g", "%Y-%m-%gT%H:%M:%S%z", "2019-1-2T15:4:5Z"},
		{"r", "%Y-%m-%d %r", "2019-1-2 3:4:5 pm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := Parse(tc.format, func(native string) (time.Time, error) { return time.Parse(native, tc.input) })
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestZulu(t *testing.T) {
	format := "%Y-%m-%dT%H:%M:%S.%L%z"
	// These time should all parse as UTC.
	for _, input := range []string{
		"2019-01-02T15:04:05.666666Z",
		"2019-01-02T15:04:05.666666-0000",
		"2019-01-02T15:04:05.666666+0000",
		"2019-01-02T15:04:05.666666+00:00",
		"2019-01-02T15:04:05.666666+00",
		"2019-01-02T15:04:05.666666 Z",
		"2019-01-02T15:04:05.666666 -0000",
		"2019-01-02T15:04:05.666666 +0000",
		"2019-01-02T15:04:05.666666 +00:00",
		"2019-01-02T15:04:05.666666 +00",
	} {
		t.Run(input, func(t *testing.T) {
			dt, layout, err := Parse(format, func(native string) (time.Time, error) { return time.Parse(native, input) })
			require.NoError(t, err)
			assert.NotEmpty(t, layout)
			// We compare the unix nanoseconds because Go has a subtle parsing difference between "Z" and "+0000".
			// The former returns a Time with the UTC timezone, the latter returns a Time with a 0000 time zone offset.
			// (See Go's documentation for `time.Parse`.)
			assert.Equal(t, dt.UnixNano(), dt1.UnixNano(), "Given: %v, expected: %v", dt, dt1)
		})
	}
}

func TestValidate(t *testing.T) {
	type args struct {
		layout string
	}
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "valid format",
			args: args{
				layout: "%Y-%m-%d %H:%M:%S.%f",
			},
			wantErr: "",
		},
		{
			name: "invalid fractional second",
			args: args{
				layout: "%Y-%m-%d-%H-%M-%S:%L",
			},
			wantErr: "invalid fractional seconds directive: ':%L'. must be preceded with '.' or ','",
		},
		{
			name: "format including decimal",
			args: args{
				layout: "2006-%m-%d-%H-%M-%S:%L",
			},
			wantErr: "format string should not contain decimals",
		},
		{
			name: "unsupported directive",
			args: args{
				layout: "%C-%m-%d-%H-%M-%S.%L",
			},
			wantErr: "invalid strptime format: [unsupported ctimefmt.toNative() directive: %C]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.args.layout)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
