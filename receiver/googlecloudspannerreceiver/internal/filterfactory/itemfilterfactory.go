// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filterfactory"

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	defaultMetricDataPointsAmountInPeriod = 24 * 60
	defaultItemActivityPeriod             = 24 * time.Hour
)

type itemFilterFactory struct {
	filterByMetric map[string]filter.ItemFilter
}

type ItemFilterFactoryConfig struct {
	MetadataItems  []*metadata.MetricsMetadata
	TotalLimit     int
	ProjectAmount  int
	InstanceAmount int
	DatabaseAmount int
}

func NewItemFilterResolver(logger *zap.Logger, config *ItemFilterFactoryConfig) (filter.ItemFilterResolver, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	builder := filterBuilder{
		logger: logger,
		config: config,
	}

	if config.TotalLimit == 0 {
		return &itemFilterFactory{
			filterByMetric: builder.buildFilterByMetricZeroTotalLimit(),
		}, nil
	}

	filterByMetric, err := builder.buildFilterByMetricPositiveTotalLimit()
	if err != nil {
		return nil, err
	}

	return &itemFilterFactory{
		filterByMetric: filterByMetric,
	}, nil
}

func (config *ItemFilterFactoryConfig) validate() error {
	if len(config.MetadataItems) == 0 {
		return errors.New("metadata items cannot be empty or nil")
	}

	if config.TotalLimit != 0 && config.TotalLimit <= (config.ProjectAmount*config.InstanceAmount*config.DatabaseAmount) {
		return errors.New("total limit is too low and doesn't cover configured projects * instances * databases")
	}

	return nil
}

func (f *itemFilterFactory) Resolve(metricFullName string) (filter.ItemFilter, error) {
	itemFilter, exists := f.filterByMetric[metricFullName]

	if !exists {
		return nil, fmt.Errorf("can't find item filter for metric with full name %q", metricFullName)
	}

	return itemFilter, nil
}

func (f *itemFilterFactory) Shutdown() error {
	for _, itemFilter := range f.filterByMetric {
		err := itemFilter.Shutdown()
		if err != nil {
			return err
		}
	}

	return nil
}
