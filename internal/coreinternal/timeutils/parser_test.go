// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseGoTimeBadLocation(t *testing.T) {
	_, err := ParseGotime(time.RFC822, "02 Jan 06 15:04 BST", time.UTC)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location BST")
}

func Test_setTimestampYear(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 06, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 06, 16, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2020, 06, 16, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("FutureOneDay", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 01, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 01, 17, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2020, 01, 17, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("FutureEightDays", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 01, 16, 3, 31, 34, 525, time.UTC)
		}

		noYear := time.Date(0, 01, 24, 3, 31, 34, 525, time.UTC)
		yearAdded := SetTimestampYear(noYear)
		expected := time.Date(2019, 01, 24, 3, 31, 34, 525, time.UTC)
		require.Equal(t, expected, yearAdded)
	})

	t.Run("RolloverYear", func(t *testing.T) {
		Now = func() time.Time {
			return time.Date(2020, 01, 01, 3, 31, 34, 525, time.UTC)
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
