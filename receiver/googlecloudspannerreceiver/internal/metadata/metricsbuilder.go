// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
)

const instrumentationLibraryName = "otelcol/googlecloudspannermetrics"

type MetricsBuilder interface {
	Build(dataPoints []*MetricsDataPoint) (pmetric.Metrics, error)
	Shutdown() error
}

type metricsFromDataPointBuilder struct {
	filterResolver filter.ItemFilterResolver
}

func NewMetricsFromDataPointBuilder(filterResolver filter.ItemFilterResolver) MetricsBuilder {
	return &metricsFromDataPointBuilder{
		filterResolver: filterResolver,
	}
}

func (b *metricsFromDataPointBuilder) Shutdown() error {
	return b.filterResolver.Shutdown()
}

func (b *metricsFromDataPointBuilder) Build(dataPoints []*MetricsDataPoint) (pmetric.Metrics, error) {
	var metrics pmetric.Metrics

	groupedDataPoints, err := b.groupAndFilter(dataPoints)
	if err != nil {
		return pmetric.Metrics{}, err
	}

	metrics = pmetric.NewMetrics()
	rms := metrics.ResourceMetrics()
	rm := rms.AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	ilm.Scope().SetName(instrumentationLibraryName)

	for key, points := range groupedDataPoints {
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName(key.MetricName)
		metric.SetUnit(key.MetricUnit)

		var dataPointSlice pmetric.NumberDataPointSlice
		switch key.MetricType.MetricType() {
		case pmetric.MetricTypeGauge:
			dataPointSlice = metric.SetEmptyGauge().DataPoints()
		case pmetric.MetricTypeSum:
			metric.SetEmptySum().SetAggregationTemporality(key.MetricType.AggregationTemporality())
			metric.Sum().SetIsMonotonic(key.MetricType.IsMonotonic())
			dataPointSlice = metric.Sum().DataPoints()
		}

		for _, point := range points {
			point.CopyTo(dataPointSlice.AppendEmpty())
		}
	}

	return metrics, nil
}

func (b *metricsFromDataPointBuilder) groupAndFilter(dataPoints []*MetricsDataPoint) (map[MetricsDataPointKey][]*MetricsDataPoint, error) {
	if len(dataPoints) == 0 {
		return nil, nil
	}

	groupedDataPoints := make(map[MetricsDataPointKey][]*MetricsDataPoint)

	for _, dataPoint := range dataPoints {
		groupingKey := dataPoint.GroupingKey()
		groupedDataPoints[groupingKey] = append(groupedDataPoints[groupingKey], dataPoint)
	}

	// Cardinality filtering
	for groupingKey, points := range groupedDataPoints {
		filteredPoints, err := b.filter(groupingKey.MetricName, points)
		if err != nil {
			return nil, err
		}

		groupedDataPoints[groupingKey] = filteredPoints
	}

	return groupedDataPoints, nil
}

func (b *metricsFromDataPointBuilder) filter(metricName string, dataPoints []*MetricsDataPoint) ([]*MetricsDataPoint, error) {
	itemFilter, err := b.filterResolver.Resolve(metricName)
	if err != nil {
		return nil, err
	}

	itemsForFiltering := make([]*filter.Item, len(dataPoints))

	for i, dataPoint := range dataPoints {
		itemsForFiltering[i], err = dataPoint.ToItem()
		if err != nil {
			return nil, err
		}
	}

	filteredItems, err := itemFilter.Filter(itemsForFiltering)
	if err != nil {
		return nil, err
	}

	// Creating new slice instead of removing elements from source slice because removing by value is not efficient operation.
	// Need to use such approach for preserving data points order.
	filteredItemsSet := make(map[filter.Item]struct{})

	for _, filteredItem := range filteredItems {
		filteredItemsSet[*filteredItem] = struct{}{}
	}

	filteredDataPoints := make([]*MetricsDataPoint, len(filteredItems))
	nextFilteredDataPointIndex := 0
	for i, dataPointItem := range itemsForFiltering {
		_, exists := filteredItemsSet[*dataPointItem]

		if exists {
			filteredDataPoints[nextFilteredDataPointIndex] = dataPoints[i]
			nextFilteredDataPointIndex++
		}
	}

	return filteredDataPoints, nil
}
