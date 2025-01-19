// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoTimeBadLocation(t *testing.T) {
	_, err := ParseGotime(time.RFC822, "02 Jan 06 15:04 BST", time.UTC)
	require.ErrorContains(t, err, "failed to load location BST")
}

func Test_setTimestampYear(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 0o6, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 0o6, 16, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2020, 0o6, 16, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("FutureOneDay", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 0o1, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 0o1, 17, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2020, 0o1, 17, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("FutureEightDays", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 0o1, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 0o1, 24, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2019, 0o1, 24, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("RolloverYear", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 0o1, 0o1, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 12, 31, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2019, 12, 31, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})
}

func TestValidateGotime(t *testing.T) {
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
				layout: "2006-01-02 15:04:05.999999",
			},
			wantErr: "",
		},
		{
			name: "valid format 2",
			args: args{
				layout: "2006-01-02 15:04:05,999999",
			},
			wantErr: "",
		},
		{
			name: "invalid fractional second",
			args: args{
				layout: "2006-01-02 15:04:05:999999",
			},
			wantErr: "invalid fractional seconds directive: ':999999'. must be preceded with '.' or ','",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGotime(tt.args.layout)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseLocalizedStrptime(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		value    any
		language string
		expected time.Time
		location *time.Location
	}{
		{
			name:     "Foreign language",
			format:   "%B %d %A, %Y, %r",
			value:    "Febrero 25 jueves, 1993, 02:03:04 p.m.",
			expected: time.Date(1993, 2, 25, 14, 3, 4, 0, time.Local),
			location: time.Local,
			language: "es-ES",
		},
		{
			name:     "Foreign language with location",
			format:   "%A %h %e %Y",
			value:    "mercoledì set 4 2024",
			expected: time.Date(2024, 9, 4, 0, 0, 0, 0, time.UTC),
			location: time.UTC,
			language: "it-IT",
		},
		{
			name:     "String value",
			format:   "%B %d %A, %Y, %I:%M:%S %p",
			value:    "March 12 Friday, 2004, 02:03:04 AM",
			expected: time.Date(2004, 3, 12, 2, 3, 4, 0, time.Local),
			location: time.Local,
			language: "en",
		},
		{
			name:     "Bytes value",
			format:   "%h %d %a, %y, %I:%M:%S %p",
			value:    []byte("Jun 10 Fri, 04, 02:03:04 AM"),
			expected: time.Date(2004, 6, 10, 2, 3, 4, 0, time.Local),
			location: time.Local,
			language: "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseLocalizedStrptime(tt.format, tt.value, tt.location, tt.language)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.UnixNano(), result.UnixNano())
		})
	}
}

func TestParseLocalizedStrptimeInvalidType(t *testing.T) {
	value := time.Now().UnixNano()
	_, err := ParseLocalizedStrptime("%c", value, time.Local, "en")
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot be parsed as a time")
}

func TestParseLocalizedGotime(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		value    any
		language string
		expected time.Time
		location *time.Location
	}{
		{
			name:     "Foreign language",
			format:   "January 02 Monday, 2006, 03:04:05 pm",
			value:    "Febrero 25 jueves, 1993, 02:03:04 p.m.",
			expected: time.Date(1993, 2, 25, 14, 3, 4, 0, time.Local),
			location: time.Local,
			language: "es-ES",
		},
		{
			name:     "Foreign language with location",
			format:   "Monday Jan _2 2006",
			value:    "mercoledì set 4 2024",
			expected: time.Date(2024, 9, 4, 0, 0, 0, 0, time.UTC),
			location: time.UTC,
			language: "it-IT",
		},
		{
			name:     "String value",
			format:   "January 02 Monday, 2006, 03:04:05 PM",
			value:    "March 12 Friday, 2004, 02:03:04 AM",
			expected: time.Date(2004, 3, 12, 2, 3, 4, 0, time.Local),
			location: time.Local,
			language: "en",
		},
		{
			name:     "Bytes value",
			format:   "Jan 02 Mon, 06, 03:04:05 PM",
			value:    []byte("Jun 10 Fri, 04, 02:03:04 AM"),
			expected: time.Date(2004, 6, 10, 2, 3, 4, 0, time.Local),
			location: time.Local,
			language: "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseLocalizedGotime(tt.format, tt.value, tt.location, tt.language)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.UnixNano(), result.UnixNano())
		})
	}
}

func TestParseLocalizedGotimeInvalidType(t *testing.T) {
	value := time.Now().UnixNano()
	_, err := ParseLocalizedStrptime("Mon", value, time.Local, "en")
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot be parsed as a time")
}

func TestValidateLocale(t *testing.T) {
	require.NoError(t, ValidateLocale("es"))
	require.NoError(t, ValidateLocale("en-US"))
	require.NoError(t, ValidateLocale("ca-ES-valencia"))
}

func TestValidateLocaleUnsupported(t *testing.T) {
	err := ValidateLocale("foo-bar")
	require.ErrorContains(t, err, "unsupported locale 'foo-bar'")
}
