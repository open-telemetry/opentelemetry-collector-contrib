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
	value1  = "2019-01-02 15:04:05.666666"
	value2  = "2019-01-02 3:04:05.666 pm, Wed"
	dt1     = time.Date(2019, 1, 2, 15, 4, 5, 666666000, time.UTC)
	dt2     = time.Date(2019, 1, 2, 15, 4, 5, 666000000, time.UTC)
)

func TestFormat(t *testing.T) {
	s, err := Format(format1, dt1)
	require.NoError(t, err)
	assert.Equal(t, s, value1, "Given: %v, expected: %v", s, value1)

	s, err = Format(format2, dt1)
	require.NoError(t, err)
	assert.Equal(t, s, value2, "Given: %v, expected: %v", s, value2)
}

func TestParse(t *testing.T) {
	dt, err := Parse(format1, value1)
	require.NoError(t, err)
	assert.Equal(t, dt, dt1, "Given: %v, expected: %v", dt, dt1)

	dt, err = Parse(format2, value2)
	require.NoError(t, err)
	assert.Equal(t, dt, dt2, "Given: %v, expected: %v", dt, dt2)
}

func TestZulu(t *testing.T) {
	format := "%Y-%m-%dT%H:%M:%S.%L%z"
	// These time should all parse as UTC.
	for _, input := range []string{
		"2019-01-02T15:04:05.666666Z",
		"2019-01-02T15:04:05.666666-0000",
		"2019-01-02T15:04:05.666666+0000",
	} {
		t.Run(input, func(t *testing.T) {
			dt, err := Parse(format, input)
			require.NoError(t, err)
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
			wantErr: "invalid strptime format: [unsupported ctimefmt.ToNative() directive: %C]",
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

func TestGetNativeSubstitutes(t *testing.T) {
	type args struct {
		format string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "get ctime directives",
			args: args{
				format: "%Y-%m-%d %H:%M:%S.%f",
			},
			want: map[string]string{"01": "%m", "02": "%d", "04": "%M", "05": "%S", "15": "%H", "2006": "%Y", "999999": "%f"},
		},
		{
			name: "format contains unsupported directive",
			args: args{
				format: "%C-%m-%d-%H-%M-%S.%L",
			},
			want: map[string]string{"01": "%m", "02": "%d", "04": "%M", "05": "%S", "15": "%H", "999": "%L"},
		},
		{
			name: "format contains Go layout elements",
			args: args{
				format: "2006-%m-%d",
			},
			want: map[string]string{"01": "%m", "02": "%d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, GetNativeSubstitutes(tt.args.format), "GetNativeSubstitutes(%v)", tt.args.format)
		})
	}
}
