// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimestampsGenerator_PullTimestamps(t *testing.T) {
	now := time.Date(2021, 9, 17, 16, 25, 30, 0, time.UTC)
	nowAtStartOfMinute := shiftToStartOfMinute(now)
	backfillIntervalAgo := nowAtStartOfMinute.Add(-1 * backfillIntervalDuration)
	backfillIntervalAgoWithSomeSeconds := backfillIntervalAgo.Add(-15 * time.Second)
	lastPullTimestampInFuture := nowAtStartOfMinute.Add(backfillIntervalDuration)

	testCases := map[string]struct {
		lastPullTimestamp  time.Time
		backfillEnabled    bool
		amountOfTimestamps int
	}{
		"Zero last pull timestamp without backfill":                                                       {time.Time{}, false, 1},
		"Zero last pull timestamp with backfill":                                                          {time.Time{}, true, int(backfillIntervalDuration.Minutes())},
		"Last pull timestamp now at start of minute backfill does not matter":                             {nowAtStartOfMinute, false, 1},
		"Last pull timestamp back fill interval ago of minute backfill does not matter":                   {backfillIntervalAgo, false, int(backfillIntervalDuration.Minutes())},
		"Last pull timestamp back fill interval ago with some seconds of minute backfill does not matter": {backfillIntervalAgoWithSomeSeconds, false, int(backfillIntervalDuration.Minutes()) + 1},
		"Last pull timestamp greater than now without backfill":                                           {lastPullTimestampInFuture, false, 1},
		"Last pull timestamp greater than now with backfill":                                              {lastPullTimestampInFuture, true, 1},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			generator := &timestampsGenerator{
				backfillEnabled: testCase.backfillEnabled,
				difference:      time.Minute,
			}
			timestamps := generator.pullTimestamps(testCase.lastPullTimestamp, now)

			assert.Equal(t, testCase.amountOfTimestamps, len(timestamps))
		})
	}
}

func TestPullTimestampsWithDifference(t *testing.T) {
	expectedAmountOfTimestamps := 5
	lowerBound := time.Date(2021, 9, 17, 16, 25, 0, 0, time.UTC)
	upperBound := lowerBound.Add(time.Duration(expectedAmountOfTimestamps) * time.Minute)

	timestamps := pullTimestampsWithDifference(lowerBound, upperBound, time.Minute)

	assert.Equal(t, expectedAmountOfTimestamps, len(timestamps))

	expectedTimestamp := lowerBound.Add(time.Minute)

	for _, timestamp := range timestamps {
		assert.Equal(t, expectedTimestamp, timestamp)
		expectedTimestamp = expectedTimestamp.Add(time.Minute)
	}

	// Check edge case: ensure that we didn't miss upperBound
	upperBound = lowerBound.Add(5 * time.Minute).Add(15 * time.Second)
	timestamps = pullTimestampsWithDifference(lowerBound, upperBound, time.Minute)

	assert.Equal(t, 6, len(timestamps))

	expectedTimestamp = lowerBound.Add(time.Minute)

	for i := 0; i < expectedAmountOfTimestamps; i++ {
		assert.Equal(t, expectedTimestamp, timestamps[i])
		expectedTimestamp = expectedTimestamp.Add(time.Minute)
	}

	assert.Equal(t, upperBound, timestamps[expectedAmountOfTimestamps])

}

func TestShiftToStartOfMinute(t *testing.T) {
	now := time.Now().UTC()
	actual := shiftToStartOfMinute(now)

	assert.Equal(t, 0, actual.Second())
	assert.Equal(t, 0, actual.Nanosecond())
}

func TestTimestampsGenerator_IsBackfillExecution(t *testing.T) {
	testCases := map[string]struct {
		lastPullTimestamp time.Time
		backfillEnabled   bool
		expectedResult    bool
	}{
		"Zero last pull timestamp with backfill":        {time.Time{}, true, true},
		"Non-zero last pull timestamp with backfill":    {time.Now().UTC(), true, false},
		"Zero last pull timestamp with no backfill":     {time.Time{}, false, false},
		"Non-zero last pull timestamp with no backfill": {time.Now().UTC(), false, false},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			generator := &timestampsGenerator{
				backfillEnabled: testCase.backfillEnabled,
			}

			assert.Equal(t, testCase.expectedResult, generator.isBackfillExecution(testCase.lastPullTimestamp))
		})
	}
}
