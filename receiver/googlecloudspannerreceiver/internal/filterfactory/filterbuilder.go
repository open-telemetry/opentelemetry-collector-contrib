// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filterfactory"

import (
	"errors"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type filterBuilder struct {
	logger *zap.Logger
	config *ItemFilterFactoryConfig
}

func (b filterBuilder) buildFilterByMetricZeroTotalLimit() map[string]filter.ItemFilter {
	filterByMetric := make(map[string]filter.ItemFilter)
	nopFilter := filter.NewNopItemCardinalityFilter()

	for _, metadataItem := range b.config.MetadataItems {
		for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
			metricFullName := metadataItem.MetricNamePrefix + metricValueMetadata.Name()
			filterByMetric[metricFullName] = nopFilter
		}
	}

	return filterByMetric
}

func (b filterBuilder) buildFilterByMetricPositiveTotalLimit() (map[string]filter.ItemFilter, error) {
	filterByMetric := make(map[string]filter.ItemFilter)
	groupedItems := groupByCardinality(b.config.MetadataItems)

	// Handle metric groups with low cardinality
	lowCardinalityGroups := groupedItems[false]
	newTotalLimit, err := b.handleLowCardinalityGroups(lowCardinalityGroups, b.config.TotalLimit, filterByMetric)
	if err != nil {
		return nil, err
	}

	// Handle metric groups with high cardinality
	highCardinalityGroups := groupedItems[true]
	newTotalLimit, err = b.handleHighCardinalityGroups(highCardinalityGroups, newTotalLimit, filterByMetric)
	if err != nil {
		return nil, err
	}

	b.logger.Debug("Remaining total limit after cardinality limits calculation",
		zap.Int("remainingTotalLimit", newTotalLimit))

	return filterByMetric, nil
}

func (b filterBuilder) handleLowCardinalityGroups(groups []*metadata.MetricsMetadata, remainingTotalLimit int,
	filterByMetric map[string]filter.ItemFilter) (int, error) {

	if len(groups) == 0 {
		return remainingTotalLimit, nil
	}

	limitPerMetricByTimestamp := b.config.ProjectAmount * b.config.InstanceAmount * b.config.DatabaseAmount

	// For low cardinality metrics total limit is equal to limit by timestamp
	b.logger.Debug("Calculated cardinality limits for low cardinality metric group",
		zap.Int("limitPerMetricByTimestamp", limitPerMetricByTimestamp))

	return b.constructFiltersForGroups(limitPerMetricByTimestamp, limitPerMetricByTimestamp, groups, remainingTotalLimit, filterByMetric)
}

func (b filterBuilder) handleHighCardinalityGroups(groups []*metadata.MetricsMetadata, remainingTotalLimit int,
	filterByMetric map[string]filter.ItemFilter) (int, error) {

	if len(groups) == 0 {
		return remainingTotalLimit, nil
	}

	totalLimitPerMetric := remainingTotalLimit / countMetricsInGroups(groups)
	limitPerMetricByTimestamp := totalLimitPerMetric / defaultMetricDataPointsAmountInPeriod

	b.logger.Debug("Calculated cardinality limits for high cardinality metric group",
		zap.Int("limitPerMetricByTimestamp", limitPerMetricByTimestamp),
		zap.Int("totalLimitPerMetric", totalLimitPerMetric))

	if limitPerMetricByTimestamp < 1 {
		return remainingTotalLimit, errors.New("limit per metric per timestamp for high cardinality metrics is lower than 1")
	}

	return b.constructFiltersForGroups(totalLimitPerMetric, limitPerMetricByTimestamp, groups, remainingTotalLimit, filterByMetric)
}

func (b filterBuilder) constructFiltersForGroups(totalLimitPerMetric int, limitPerMetricByTimestamp int,
	groups []*metadata.MetricsMetadata, remainingTotalLimit int, filterByMetric map[string]filter.ItemFilter) (int, error) {

	newTotalLimit := remainingTotalLimit

	for _, metadataItem := range groups {
		for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
			newTotalLimit -= totalLimitPerMetric
			metricFullName := metadataItem.MetricNamePrefix + metricValueMetadata.Name()

			b.logger.Debug("Setting cardinality limits for metric",
				zap.String("metricFullName", metricFullName),
				zap.Int("limitPerMetricByTimestamp", limitPerMetricByTimestamp),
				zap.Int("totalLimitPerMetric", totalLimitPerMetric),
				zap.Int("remainingTotalLimit", newTotalLimit))

			itemFilter, err := filter.NewItemCardinalityFilter(metricFullName, totalLimitPerMetric,
				limitPerMetricByTimestamp, defaultItemActivityPeriod, b.logger)
			if err != nil {
				return remainingTotalLimit, err
			}
			filterByMetric[metricFullName] = itemFilter
		}
	}

	return newTotalLimit, nil
}

func countMetricsInGroups(metadataItems []*metadata.MetricsMetadata) (amount int) {
	for _, metadataItem := range metadataItems {
		amount += len(metadataItem.QueryMetricValuesMetadata)
	}

	return amount
}

func groupByCardinality(metadataItems []*metadata.MetricsMetadata) map[bool][]*metadata.MetricsMetadata {
	groupedItems := make(map[bool][]*metadata.MetricsMetadata)

	for _, metadataItem := range metadataItems {
		groupedItems[metadataItem.HighCardinality] = append(groupedItems[metadataItem.HighCardinality], metadataItem)
	}

	return groupedItems
}
