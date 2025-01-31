// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
)

type Item struct {
	SeriesKey string
	Timestamp time.Time
}

type ItemFilter interface {
	Filter(source []*Item) []*Item
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
	cache              *ttlcache.Cache[string, struct{}]
	stopOnce           sync.Once
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
	itemActivityPeriod time.Duration, logger *zap.Logger,
) (ItemFilter, error) {
	if limitByTimestamp > totalLimit {
		return nil, fmt.Errorf("total limit %q is lower or equal to limit by timestamp %q", totalLimit, limitByTimestamp)
	}

	cache := ttlcache.New[string, struct{}](
		ttlcache.WithCapacity[string, struct{}](uint64(totalLimit)),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
	)
	go cache.Start()

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

func (f *itemCardinalityFilter) Filter(sourceItems []*Item) []*Item {
	var filteredItems []*Item
	groupedItems := groupByTimestamp(sourceItems)
	sortedItemKeys := sortedKeys(groupedItems)

	for _, key := range sortedItemKeys {
		filteredItems = append(filteredItems, f.filterItems(groupedItems[key])...)
	}

	return filteredItems
}

func (f *itemCardinalityFilter) filterItems(items []*Item) []*Item {
	limit := currentLimitByTimestamp{
		limitByTimestamp: f.limitByTimestamp,
	}

	var filteredItems []*Item
	for _, item := range items {
		if f.includeItem(item, &limit) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems
}

func (f *itemCardinalityFilter) includeItem(item *Item, limit *currentLimitByTimestamp) bool {
	if f.cache.Get(item.SeriesKey) != nil {
		return true
	}
	if !f.canIncludeNewItem(limit.get()) {
		f.logger.Debug("Skip item", zap.String("seriesKey", item.SeriesKey), zap.Time("timestamp", item.Timestamp))
		return false
	}

	_ = f.cache.Set(item.SeriesKey, struct{}{}, f.itemActivityPeriod)

	f.logger.Debug("Added item to cache", zap.String("seriesKey", item.SeriesKey), zap.Time("timestamp", item.Timestamp))

	limit.dec()

	return true
}

func (f *itemCardinalityFilter) canIncludeNewItem(currentLimitByTimestamp int) bool {
	return f.cache.Len() < f.totalLimit && currentLimitByTimestamp > 0
}

func (f *itemCardinalityFilter) Shutdown() error {
	f.stopOnce.Do(func() { f.cache.Stop() })
	return nil
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
