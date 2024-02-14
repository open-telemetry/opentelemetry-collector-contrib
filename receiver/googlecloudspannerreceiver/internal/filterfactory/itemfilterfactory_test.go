// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

func TestNewItemFilterResolver(t *testing.T) {
	logger := zaptest.NewLogger(t)
	metricPrefixes := []string{prefix1, prefix2}
	prefixHighCardinality := []bool{true, true}
	metadataItems := generateMetadataItems(metricPrefixes, prefixHighCardinality)
	testCases := map[string]struct {
		totalLimit  int
		expectError bool
	}{
		"Total limit is zero":                          {0, false},
		"Total limit is positive":                      {200 * defaultMetricDataPointsAmountInPeriod, false},
		"Total limit is lover then product of amounts": {3, true},
		"Error when limit by timestamp is lower than 1 for high cardinality groups": {20, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			config := &ItemFilterFactoryConfig{
				MetadataItems:  metadataItems,
				TotalLimit:     testCase.totalLimit,
				ProjectAmount:  1,
				InstanceAmount: 2,
				DatabaseAmount: 5,
			}

			factory, err := NewItemFilterResolver(logger, config)

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, factory)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestItemFilterFactoryConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		metadataItems  []*metadata.MetricsMetadata
		totalLimit     int
		projectAmount  int
		instanceAmount int
		databaseAmount int
		expectError    bool
	}{
		"No metadata items":                            {[]*metadata.MetricsMetadata{}, 10, 1, 1, 1, true},
		"Total limit is zero":                          {[]*metadata.MetricsMetadata{{}}, 0, 1, 1, 1, false},
		"Total limit is lover then product of amounts": {[]*metadata.MetricsMetadata{{}}, 3, 1, 2, 3, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			config := &ItemFilterFactoryConfig{
				MetadataItems:  testCase.metadataItems,
				TotalLimit:     testCase.totalLimit,
				ProjectAmount:  testCase.projectAmount,
				InstanceAmount: testCase.instanceAmount,
				DatabaseAmount: testCase.databaseAmount,
			}

			err := config.validate()

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestItemFilterFactory_Resolve(t *testing.T) {
	itemFilter := filter.NewNopItemCardinalityFilter()
	testCases := map[string]struct {
		filterByMetric map[string]filter.ItemFilter
		expectError    bool
	}{
		"Filter cannot be resolved": {map[string]filter.ItemFilter{}, true},
		"Filter can be resolved":    {map[string]filter.ItemFilter{metricFullName: itemFilter}, false},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			factory := &itemFilterFactory{
				filterByMetric: testCase.filterByMetric,
			}

			resolvedFilter, err := factory.Resolve(metricFullName)

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, resolvedFilter)
			} else {
				require.NoError(t, err)
				assert.Equal(t, itemFilter, resolvedFilter)
			}
		})
	}
}

func TestItemFilterFactory_Shutdown(t *testing.T) {
	testCases := map[string]struct {
		expectedError error
	}{
		"Error":      {errors.New("error on shutdown")},
		"Happy path": {nil},
	}

	for name, testCase := range testCases {
		mf := &mockFilter{}
		t.Run(name, func(t *testing.T) {
			factory := &itemFilterFactory{
				filterByMetric: map[string]filter.ItemFilter{metricFullName: mf},
			}

			mf.On("Shutdown").Return(testCase.expectedError)

			_ = factory.Shutdown()

			mf.AssertExpectations(t)
		})
	}
}
