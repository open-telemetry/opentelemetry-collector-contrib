// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewItemCardinalityFilter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		totalLimit       int
		limitByTimestamp int
		expectError      bool
	}{
		"Happy path": {2, 1, false},
		"Overall limit is lower than limit by timestamp": {1, 2, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filter, err := NewItemCardinalityFilter(metricName, testCase.totalLimit, testCase.limitByTimestamp,
				itemActivityPeriod, logger)

			if testCase.expectError {
				require.Nil(t, filter)
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				filterCasted := filter.(*itemCardinalityFilter)
				defer executeShutdown(t, filterCasted)

				assert.Equal(t, metricName, filterCasted.metricName)
				assert.Equal(t, testCase.totalLimit, filterCasted.totalLimit)
				assert.Equal(t, testCase.limitByTimestamp, filterCasted.limitByTimestamp)
				assert.Equal(t, itemActivityPeriod, filterCasted.itemActivityPeriod)
				assert.Equal(t, logger, filterCasted.logger)
				require.NotNil(t, filterCasted.cache)
			}
		})
	}
}

func TestItemCardinalityFilter_TotalLimit(t *testing.T) {
	itemFilter := &itemCardinalityFilter{totalLimit: totalLimit}

	assert.Equal(t, totalLimit, itemFilter.TotalLimit())
}

func TestItemCardinalityFilter_LimitByTimestamp(t *testing.T) {
	itemFilter := &itemCardinalityFilter{limitByTimestamp: limitByTimestamp}

	assert.Equal(t, limitByTimestamp, itemFilter.LimitByTimestamp())
}

func TestItemCardinalityFilter_CanIncludeNewItem(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		totalLimit         int
		limitByTimestamp   int
		keysAlreadyInCache []string
		expectedResult     bool
	}{
		"No items in cache and timestamp limit hasn't been exhausted": {2, 1, nil, true},
		"Cache is full and timestamp limit hasn't been exhausted":     {2, 1, []string{"qwerty1", "qwerty2"}, false},
		"No items in cache and timestamp limit has been exhausted":    {2, 0, nil, false},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filter, err := NewItemCardinalityFilter(metricName, testCase.totalLimit, testCase.limitByTimestamp,
				itemActivityPeriod, logger)
			require.NoError(t, err)
			filterCasted := filter.(*itemCardinalityFilter)
			defer executeShutdown(t, filterCasted)

			for _, key := range testCase.keysAlreadyInCache {
				err = filterCasted.cache.Set(key, byte(1))
				require.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedResult, filterCasted.canIncludeNewItem(testCase.limitByTimestamp))
		})
	}
}

func TestItemCardinalityFilter_Shutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		closeCache  bool
		expectError bool
	}{
		"Happy path":            {false, false},
		"Cache has been closed": {true, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
			require.NoError(t, err)
			filterCasted := filter.(*itemCardinalityFilter)

			if testCase.closeCache {
				// Covering case when by some reasons cache is closed
				err = filterCasted.cache.Close()
				require.NoError(t, err)
			}

			if testCase.expectError {
				require.Error(t, filter.Shutdown())
			} else {
				require.NoError(t, filter.Shutdown())
			}
		})
	}
}

func TestItemCardinalityFilter_Filter(t *testing.T) {
	items := initialItems(t)
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted := filter.(*itemCardinalityFilter)
	defer executeShutdown(t, filterCasted)

	filteredItems, err := filter.Filter(items)
	require.NoError(t, err)

	// Items with key3 and key6 must be not present in filtered items
	assertInitialFiltering(t, expectedFilteredInitialItems(t), filteredItems)

	items = additionalTestData(t)
	filteredItems, err = filter.Filter(items)
	require.NoError(t, err)

	// Cache timeout hasn't been reached, so filtered out all items
	assert.Equal(t, 0, len(filteredItems))

	// Doing this to avoid of relying on timeouts and sleeps(avoid potential flaky tests)
	syncChannel := make(chan bool)

	filterCasted.cache.SetExpirationCallback(func(key string, value any) {
		if filterCasted.cache.Count() > 0 {
			// Waiting until cache is really empty - all items are expired
			return
		}
		syncChannel <- true
	})

	<-syncChannel

	filterCasted.cache.SetExpirationCallback(nil)

	filteredItems, err = filter.Filter(items)
	require.NoError(t, err)

	// All entries expired, nothing should be filtered out from items
	assertInitialFiltering(t, items, filteredItems)

	// Test filtering when cache was closed
	filter, err = NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	require.NoError(t, filter.Shutdown())

	filteredItems, err = filter.Filter(items)

	require.Error(t, err)
	require.Nil(t, filteredItems)
}

func TestItemCardinalityFilter_FilterItems(t *testing.T) {
	items := initialItemsWithSameTimestamp(t)
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted := filter.(*itemCardinalityFilter)
	defer executeShutdown(t, filterCasted)

	filteredItems, err := filterCasted.filterItems(items)
	require.NoError(t, err)

	// Items with key1 and key2 must be not present in filtered items
	assertInitialFiltering(t, expectedFilteredInitialItemsWithSameTimestamp(t), filteredItems)

	// 2 new and 2 existing items must be present in filtered items
	filteredItems, err = filterCasted.filterItems(items)
	require.NoError(t, err)

	assert.Equal(t, totalLimit, len(filteredItems))

	filteredItems, err = filter.Filter(items)
	require.NoError(t, err)

	// Cache timeout hasn't been reached, so no more new items expected
	assert.Equal(t, totalLimit, len(filteredItems))

	// Doing this to avoid of relying on timeouts and sleeps(avoid potential flaky tests)
	syncChannel := make(chan bool)

	filterCasted.cache.SetExpirationCallback(func(key string, value any) {
		if filterCasted.cache.Count() > 0 {
			// Waiting until cache is really empty - all items are expired
			return
		}
		syncChannel <- true
	})

	<-syncChannel

	filterCasted.cache.SetExpirationCallback(nil)

	filteredItems, err = filter.Filter(items)
	require.NoError(t, err)

	// All entries expired, same picture as on first case
	assertInitialFiltering(t, expectedFilteredInitialItemsWithSameTimestamp(t), filteredItems)

	// Test filtering when cache was closed
	filter, err = NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	require.NoError(t, filter.Shutdown())

	filteredItems, err = filter.Filter(items)

	require.Error(t, err)
	require.Nil(t, filteredItems)
}

func TestItemCardinalityFilter_IncludeItem(t *testing.T) {
	timestamp := time.Now().UTC()
	item1 := &Item{SeriesKey: key1, Timestamp: timestamp}
	item2 := &Item{SeriesKey: key2, Timestamp: timestamp}
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted := filter.(*itemCardinalityFilter)
	defer executeShutdown(t, filterCasted)
	timestampLimiter := &currentLimitByTimestamp{
		limitByTimestamp: 1,
	}

	result, err := filterCasted.includeItem(item1, timestampLimiter)
	require.NoError(t, err)
	assert.True(t, result)

	// Item already exists in cache
	result, err = filterCasted.includeItem(item1, timestampLimiter)
	require.NoError(t, err)
	assert.True(t, result)

	// Limit by timestamp reached
	result, err = filterCasted.includeItem(item2, timestampLimiter)
	require.NoError(t, err)
	assert.False(t, result)

	// Test with closed cache - do not need to execute shutdown in this case
	filter, err = NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted = filter.(*itemCardinalityFilter)
	require.NoError(t, filterCasted.cache.Close())
	result, err = filterCasted.includeItem(item1, timestampLimiter)
	require.Error(t, err)
	assert.False(t, result)
}

func TestGroupByTimestamp(t *testing.T) {
	timestamp1, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)
	timestamp2, err := time.Parse(timestampLayout, timestamp2Str)
	require.NoError(t, err)
	timestamp3, err := time.Parse(timestampLayout, timestamp3Str)
	require.NoError(t, err)

	items := initialItems(t)
	groupedItems := groupByTimestamp(items)

	assert.Equal(t, 3, len(groupedItems))
	assertGroupedByKey(t, items, groupedItems, timestamp1, 0)
	assertGroupedByKey(t, items, groupedItems, timestamp2, 3)
	assertGroupedByKey(t, items, groupedItems, timestamp3, 6)
}

func TestSortedKeys(t *testing.T) {
	timestamp1, err := time.Parse(timestampLayout, timestamp1Str)
	require.NoError(t, err)
	timestamp2, err := time.Parse(timestampLayout, timestamp2Str)
	require.NoError(t, err)
	timestamp3, err := time.Parse(timestampLayout, timestamp3Str)
	require.NoError(t, err)

	data := map[time.Time][]*Item{
		timestamp3: {{SeriesKey: key3, Timestamp: timestamp3}},
		timestamp1: {{SeriesKey: key1, Timestamp: timestamp1}},
		timestamp2: {{SeriesKey: key2, Timestamp: timestamp2}},
	}

	keys := sortedKeys(data)

	assert.Equal(t, len(data), len(keys))
	assert.Equal(t, timestamp1, keys[0])
	assert.Equal(t, timestamp2, keys[1])
	assert.Equal(t, timestamp3, keys[2])
}

func TestCurrentLimitByTimestamp_Get(t *testing.T) {
	timestampLimiter := &currentLimitByTimestamp{
		limitByTimestamp: limitByTimestamp,
	}

	assert.Equal(t, limitByTimestamp, timestampLimiter.get())
}

func TestCurrentLimitByTimestamp_Dec(t *testing.T) {
	timestampLimiter := &currentLimitByTimestamp{
		limitByTimestamp: limitByTimestamp,
	}

	timestampLimiter.dec()

	assert.Equal(t, limitByTimestamp-1, timestampLimiter.limitByTimestamp)
}

func executeShutdown(t *testing.T, filter *itemCardinalityFilter) {
	require.NoError(t, filter.Shutdown())
}
