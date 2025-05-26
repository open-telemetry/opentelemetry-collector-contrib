// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
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
				item := filterCasted.cache.Set(key, struct{}{}, time.Duration(0))
				assert.NotNil(t, item)
			}

			assert.Equal(t, testCase.expectedResult, filterCasted.canIncludeNewItem(testCase.limitByTimestamp))
		})
	}
}

func TestItemCardinalityFilter_Shutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)

	// Ensure shutdown is safe to be called multiple times.
	require.NoError(t, filter.Shutdown())
	require.NoError(t, filter.Shutdown())
}

func TestItemCardinalityFilter_Filter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows due to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32397")
	}
	items := initialItems(t)
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted := filter.(*itemCardinalityFilter)
	defer executeShutdown(t, filterCasted)

	filteredItems := filter.Filter(items)

	// Items with key3 and key6 must be not present in filtered items
	assertInitialFiltering(t, expectedFilteredInitialItems(t), filteredItems)

	items = additionalTestData(t)
	filteredItems = filter.Filter(items)

	// Cache timeout hasn't been reached, so filtered out all items
	assert.Empty(t, filteredItems)

	// Doing this to avoid of relying on timeouts and sleeps(avoid potential flaky tests)
	syncChannel := make(chan bool, 10)

	filterCasted.cache.OnEviction(func(context.Context, ttlcache.EvictionReason, *ttlcache.Item[string, struct{}]) {
		if filterCasted.cache.Len() > 0 {
			// Waiting until cache is really empty - all items are expired
			return
		}
		syncChannel <- true
	})

	<-syncChannel

	filteredItems = filter.Filter(items)

	// All entries expired, nothing should be filtered out from items
	assertInitialFiltering(t, items, filteredItems)
}

func TestItemCardinalityFilter_FilterItems(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows due to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32397")
	}
	items := initialItemsWithSameTimestamp(t)
	logger := zaptest.NewLogger(t)
	filter, err := NewItemCardinalityFilter(metricName, totalLimit, limitByTimestamp, itemActivityPeriod, logger)
	require.NoError(t, err)
	filterCasted := filter.(*itemCardinalityFilter)
	defer executeShutdown(t, filterCasted)

	filteredItems := filterCasted.filterItems(items)

	// Items with key1 and key2 must be not present in filtered items
	assertInitialFiltering(t, expectedFilteredInitialItemsWithSameTimestamp(t), filteredItems)

	// 2 new and 2 existing items must be present in filtered items
	filteredItems = filterCasted.filterItems(items)

	assert.Len(t, filteredItems, totalLimit)

	filteredItems = filter.Filter(items)

	// Cache timeout hasn't been reached, so no more new items expected
	assert.Len(t, filteredItems, totalLimit)

	// Doing this to avoid of relying on timeouts and sleeps(avoid potential flaky tests)
	syncChannel := make(chan bool, 10)

	filterCasted.cache.OnEviction(func(context.Context, ttlcache.EvictionReason, *ttlcache.Item[string, struct{}]) {
		if filterCasted.cache.Len() > 0 {
			// Waiting until cache is really empty - all items are expired
			return
		}
		syncChannel <- true
	})

	<-syncChannel

	filteredItems = filter.Filter(items)

	// All entries expired, same picture as on first case
	assertInitialFiltering(t, expectedFilteredInitialItemsWithSameTimestamp(t), filteredItems)
}

func TestItemCardinalityFilter_IncludeItem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows due to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32397")
	}
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

	result := filterCasted.includeItem(item1, timestampLimiter)
	assert.True(t, result)

	// Item already exists in cache
	result = filterCasted.includeItem(item1, timestampLimiter)
	assert.True(t, result)

	// Limit by timestamp reached
	result = filterCasted.includeItem(item2, timestampLimiter)
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

	assert.Len(t, groupedItems, 3)
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

	assert.Len(t, keys, len(data))
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
