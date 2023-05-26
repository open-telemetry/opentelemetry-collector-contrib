// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package timeutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseGoTimeBadLocation(t *testing.T) {
	_, err := ParseGoTime(time.RFC822, "02 Jan 06 15:04 BST", time.UTC)
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
