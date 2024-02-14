// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	key1 = "key1"
	key2 = "key2"
	key3 = "key3"
	key4 = "key4"
	key5 = "key5"
	key6 = "key6"

	metricName = "metricName"

	totalLimit       = 4
	limitByTimestamp = 2

	itemActivityPeriod = 50 * time.Millisecond

	timestampLayout = "2006-01-02T15:04:05.000Z"

	timestamp1Str = "2021-10-13T20:30:00.000Z"
	timestamp2Str = "2021-10-13T20:30:05.000Z"
	timestamp3Str = "2021-10-13T20:30:10.000Z"
	timestamp4Str = "2021-10-13T20:30:20.000Z"
)

func assertGroupedByKey(t *testing.T, items []*Item, groupedItems map[time.Time][]*Item, key time.Time, offsetInItems int) {
	assert.Equal(t, 3, len(groupedItems[key]))

	for i := 0; i < 3; i++ {
		assert.Equal(t, items[i+offsetInItems].SeriesKey, groupedItems[key][i].SeriesKey)
	}
}

func assertInitialFiltering(t *testing.T, expected []*Item, actual []*Item) {
	require.Equal(t, len(expected), len(actual))
	for i, expectedItem := range expected {
		assert.Equal(t, expectedItem.SeriesKey, actual[i].SeriesKey)
		assert.Equal(t, expectedItem.Timestamp, actual[i].Timestamp)
	}
}

func initialItems(t *testing.T) []*Item {
	timestamp1, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)
	timestamp2, err := time.Parse(timestampLayout, timestamp2Str)
	require.NoError(t, err)
	timestamp3, err := time.Parse(timestampLayout, timestamp3Str)
	require.NoError(t, err)

	data := []*Item{
		{SeriesKey: key1, Timestamp: timestamp1},
		{SeriesKey: key2, Timestamp: timestamp1},
		{SeriesKey: key3, Timestamp: timestamp1},

		{SeriesKey: key1, Timestamp: timestamp2},
		{SeriesKey: key2, Timestamp: timestamp2},
		{SeriesKey: key5, Timestamp: timestamp2},

		{SeriesKey: key4, Timestamp: timestamp3},
		{SeriesKey: key5, Timestamp: timestamp3},
		{SeriesKey: key6, Timestamp: timestamp3},
	}

	return data
}

func expectedFilteredInitialItems(t *testing.T) []*Item {
	timestamp1, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)
	timestamp2, err := time.Parse(timestampLayout, timestamp2Str)
	require.NoError(t, err)
	timestamp3, err := time.Parse(timestampLayout, timestamp3Str)
	require.NoError(t, err)

	data := []*Item{
		{SeriesKey: key1, Timestamp: timestamp1},
		{SeriesKey: key2, Timestamp: timestamp1},

		{SeriesKey: key1, Timestamp: timestamp2},
		{SeriesKey: key2, Timestamp: timestamp2},
		{SeriesKey: key5, Timestamp: timestamp2},

		{SeriesKey: key4, Timestamp: timestamp3},
		{SeriesKey: key5, Timestamp: timestamp3},
	}

	return data
}

func additionalTestData(t *testing.T) []*Item {
	timestamp, err := time.Parse(timestampLayout, timestamp4Str)
	require.NoError(t, err)

	data := []*Item{
		{SeriesKey: key3, Timestamp: timestamp},
		{SeriesKey: key6, Timestamp: timestamp},
	}

	return data
}

func initialItemsWithSameTimestamp(t *testing.T) []*Item {
	timestamp, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)

	data := []*Item{
		{SeriesKey: key1, Timestamp: timestamp},
		{SeriesKey: key2, Timestamp: timestamp},
		{SeriesKey: key3, Timestamp: timestamp},
		{SeriesKey: key4, Timestamp: timestamp},
		{SeriesKey: key5, Timestamp: timestamp},
		{SeriesKey: key6, Timestamp: timestamp},
	}

	return data
}

func expectedFilteredInitialItemsWithSameTimestamp(t *testing.T) []*Item {
	timestamp, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)

	data := []*Item{
		{SeriesKey: key1, Timestamp: timestamp},
		{SeriesKey: key2, Timestamp: timestamp},
	}

	return data
}
