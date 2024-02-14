// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"go.uber.org/zap"
)

type Item struct {
	SeriesKey string
	Timestamp time.Time
}

type ItemFilter interface {
	Filter(source []*Item) ([]*Item, error)
	Shutdown() error
	TotalLimit() int
	LimitByTimestamp() int
}

type ItemFilterResolver interface {
	Resolve(metricFullName string) (ItemFilter, error)
	Shutdown() error
}

type itemCardinalityFilter struct {
	metricName         string
	totalLimit         int
	limitByTimestamp   int
	itemActivityPeriod time.Duration
	logger             *zap.Logger
	cache              *ttlcache.Cache
}

type currentLimitByTimestamp struct {
	limitByTimestamp int
}

func (f *currentLimitByTimestamp) dec() {
	f.limitByTimestamp--
}

func (f *currentLimitByTimestamp) get() int {
	return f.limitByTimestamp
}

func NewItemCardinalityFilter(metricName string, totalLimit int, limitByTimestamp int,
	itemActivityPeriod time.Duration, logger *zap.Logger) (ItemFilter, error) {
	if limitByTimestamp > totalLimit {
		return nil, fmt.Errorf("total limit %q is lower or equal to limit by timestamp %q", totalLimit, limitByTimestamp)
	}

	cache := ttlcache.NewCache()

	cache.SetCacheSizeLimit(totalLimit)
	cache.SkipTTLExtensionOnHit(true)

	return &itemCardinalityFilter{
		metricName:         metricName,
		totalLimit:         totalLimit,
		limitByTimestamp:   limitByTimestamp,
		itemActivityPeriod: itemActivityPeriod,
		logger:             logger,
		cache:              cache,
	}, nil
}

func (f *itemCardinalityFilter) TotalLimit() int {
	return f.totalLimit
}

func (f *itemCardinalityFilter) LimitByTimestamp() int {
	return f.limitByTimestamp
}

func (f *itemCardinalityFilter) Filter(sourceItems []*Item) ([]*Item, error) {
	var filteredItems []*Item
	groupedItems := groupByTimestamp(sourceItems)
	sortedItemKeys := sortedKeys(groupedItems)

	for _, key := range sortedItemKeys {
		filteredGroupedItems, err := f.filterItems(groupedItems[key])
		if err != nil {
			return nil, err
		}

		filteredItems = append(filteredItems, filteredGroupedItems...)
	}

	return filteredItems, nil
}

func (f *itemCardinalityFilter) filterItems(items []*Item) ([]*Item, error) {
	limit := currentLimitByTimestamp{
		limitByTimestamp: f.limitByTimestamp,
	}

	var filteredItems []*Item
	for _, item := range items {
		if included, err := f.includeItem(item, &limit); err != nil {
			return nil, err
		} else if included {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

func (f *itemCardinalityFilter) includeItem(item *Item, limit *currentLimitByTimestamp) (bool, error) {
	if _, err := f.cache.Get(item.SeriesKey); err == nil {
		return true, nil
	} else if !errors.Is(err, ttlcache.ErrNotFound) {
		return false, err
	}

	if !f.canIncludeNewItem(limit.get()) {
		f.logger.Debug("Skip item", zap.String("seriesKey", item.SeriesKey), zap.Time("timestamp", item.Timestamp))
		return false, nil
	}

	if err := f.cache.SetWithTTL(item.SeriesKey, struct{}{}, f.itemActivityPeriod); err != nil {
		if errors.Is(err, ttlcache.ErrClosed) {
			err = fmt.Errorf("set item from cache failed for metric %q because cache has been already closed: %w", f.metricName, err)
		}
		return false, err
	}

	f.logger.Debug("Added item to cache", zap.String("seriesKey", item.SeriesKey), zap.Time("timestamp", item.Timestamp))

	limit.dec()

	return true, nil
}

func (f *itemCardinalityFilter) canIncludeNewItem(currentLimitByTimestamp int) bool {
	return f.cache.Count() < f.totalLimit && currentLimitByTimestamp > 0
}

func (f *itemCardinalityFilter) Shutdown() error {
	return f.cache.Close()
}

func groupByTimestamp(items []*Item) map[time.Time][]*Item {
	groupedItems := make(map[time.Time][]*Item)

	for _, item := range items {
		groupedItems[item.Timestamp] = append(groupedItems[item.Timestamp], item)
	}

	return groupedItems
}

func sortedKeys(groupedItems map[time.Time][]*Item) []time.Time {
	keysForSorting := make([]time.Time, len(groupedItems))

	i := 0
	for key := range groupedItems {
		keysForSorting[i] = key
		i++
	}

	sort.Slice(keysForSorting, func(i, j int) bool { return keysForSorting[i].Before(keysForSorting[j]) })

	return keysForSorting
}
