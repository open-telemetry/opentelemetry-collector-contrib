// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

func getMetricsQueryResponseMockData() []queryResourcesResponseMock {
	return []queryResourcesResponseMock{
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId3",
				metricNamespace: "type1",
				metricNames:     []string{"metric7"},
				resourceIDs:     []string{"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1"},
			},
			response: newQueryResourcesResponseMockData([]queryResourceMockInput{
				{
					ResourceID: "/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1",
					Metrics: []metricMockInput{
						{
							Name: "metric7",
							Unit: azmetrics.MetricUnitBitsPerSecond,
							TimeSeries: []azmetrics.TimeSeriesElement{{
								Data: []azmetrics.MetricValue{
									{
										// Send only timestamp with all other values nil is a case that can
										// happen in the Azure responses.
										TimeStamp: to.Ptr(time.Now()),
									},
									{
										TimeStamp: to.Ptr(time.Now()),
										// Keep only Total to make sure that all values are considered.
										// Not only Average
										Total: to.Ptr(1.),
									},
								},
							}},
						},
					},
				},
			}),
		},
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId1",
				metricNamespace: "type1",
				metricNames:     []string{"metric1", "metric2"},
				resourceIDs: []string{
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
				},
			},
			response: newQueryResourcesResponseMockData([]queryResourceMockInput{
				{
					ResourceID: "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
					Metrics: []metricMockInput{
						{
							Name: "metric1",
							Unit: azmetrics.MetricUnitPercent,
							TimeSeries: []azmetrics.TimeSeriesElement{{
								Data: []azmetrics.MetricValue{{
									TimeStamp: to.Ptr(time.Now()),
									Average:   to.Ptr(1.),
									Count:     to.Ptr(1.),
									Maximum:   to.Ptr(1.),
									Minimum:   to.Ptr(1.),
									Total:     to.Ptr(1.),
								}},
							}},
						},
						{
							Name: "metric2",
							Unit: azmetrics.MetricUnitCount,
							TimeSeries: []azmetrics.TimeSeriesElement{{
								Data: []azmetrics.MetricValue{{
									TimeStamp: to.Ptr(time.Now()),
									Average:   to.Ptr(1.),
									Count:     to.Ptr(1.),
									Maximum:   to.Ptr(1.),
									Minimum:   to.Ptr(1.),
									Total:     to.Ptr(1.),
								}},
							}},
						},
					},
				},
			}),
		},
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId1",
				metricNamespace: "type1",
				metricNames:     []string{"metric3"},
				resourceIDs: []string{
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
				},
			},
			response: newQueryResourcesResponseMockData([]queryResourceMockInput{
				{
					ResourceID: "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
					Metrics: []metricMockInput{
						{
							Name: "metric3",
							Unit: azmetrics.MetricUnitBytes,
							TimeSeries: []azmetrics.TimeSeriesElement{{
								Data: []azmetrics.MetricValue{{
									TimeStamp: to.Ptr(time.Now()),
									Average:   to.Ptr(1.),
									Count:     to.Ptr(1.),
									Maximum:   to.Ptr(1.),
									Minimum:   to.Ptr(1.),
									Total:     to.Ptr(1.),
								}},
							}},
						},
					},
				},
			}),
		},
	}
}

func TestAzureScraperBatchScrape(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx context.Context
	}
	cfg := createDefaultTestConfig()
	cfg.MaximumNumberOfMetricsInACall = 2
	cfg.AppendTagsAsAttributes = []string{}
	cfg.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgTagsSelective := createDefaultTestConfig()
	cfgTagsSelective.AppendTagsAsAttributes = []string{"tagName1"}
	cfgTagsSelective.MaximumNumberOfMetricsInACall = 2
	cfgTagsSelective.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgTagsCaseInsensitive := createDefaultTestConfig()
	cfgTagsCaseInsensitive.AppendTagsAsAttributes = []string{"TAGNAME1"}
	cfgTagsCaseInsensitive.MaximumNumberOfMetricsInACall = 2
	cfgTagsCaseInsensitive.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgTagsEnabled := createDefaultTestConfig()
	cfgTagsEnabled.AppendTagsAsAttributes = []string{"*"}
	cfgTagsEnabled.MaximumNumberOfMetricsInACall = 2

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "metrics_golden",
			fields: fields{
				cfg: cfg,
			},
			args: args{
				ctx: t.Context(),
			},
		},
		{
			name: "metrics_tags_golden",
			fields: fields{
				cfg: cfgTagsEnabled,
			},
			args: args{
				ctx: t.Context(),
			},
		},
		{
			name: "metrics_selective_tags",
			fields: fields{
				cfg: cfgTagsSelective,
			},
			args: args{
				ctx: t.Context(),
			},
		},
		{
			name: "metrics_selective_tags",
			fields: fields{
				cfg: cfgTagsCaseInsensitive,
			},
			args: args{
				ctx: t.Context(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopSettings(metadata.Type)

			optionsResolver := newMockClientOptionsResolver(
				getSubscriptionByIDMockData(),
				getSubscriptionsMockData(),
				getResourcesMockData(),
				getMetricsDefinitionsMockData(),
				nil,
				getMetricsQueryResponseMockData(),
			)

			s := &azureBatchScraper{
				cfg:                          tt.fields.cfg,
				mbs:                          newConcurrentMapImpl[*metadata.MetricsBuilder](),
				mutex:                        &sync.Mutex{},
				time:                         getTimeMock(),
				clientOptionsResolver:        optionsResolver,
				receiverSettings:             settings,
				settings:                     settings.TelemetrySettings,
				storageAccountSpecificConfig: newStorageAccountSpecificConfig(tt.fields.cfg.Services),

				// From there, initialize everything that is normally initialized in start() func
				subscriptions: map[string]*azureSubscription{},
				resources:     map[string]map[string]*azureResource{},
				regions:       map[string]map[string]struct{}{},
				resourceTypes: map[string]map[string]*azureType{},
			}

			metrics, err := s.scrape(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			expectedFile := filepath.Join("testdata", "expected_metrics_batch", tt.name+".yaml")
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(
				expectedMetrics,
				metrics,
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreResourceMetricsOrder(),
			))
		})
	}
}

// TestAzureScraperBatchScrape_NoDuplicateOnRescrape is a regression test for the
// "duplicate sample for timestamp" 409 errors emitted by Prometheus-compatible
// backends (Thanos/Mimir/Cortex) when the batch scraper was re-emitting the
// same Azure timestamps on every scrape.
//
// The batch scraper emits points using the original Azure timestamp
// (cf. processQueryTimeseriesData). Without a per-(resourceType, compositeKey)
// temporal guard, calling scrape() faster than the metric's timeGrain would
// republish the exact same (labels, ts) tuples. This test calls scrape()
// twice back-to-back; with the guard in place the second call must not emit
// any data point because the configured timeGrains (PT1M / PT1H in the mocks)
// are far larger than the wall-clock interval between the two calls.
func TestAzureScraperBatchScrape_NoDuplicateOnRescrape(t *testing.T) {
	cfg := createDefaultTestConfig()
	cfg.MaximumNumberOfMetricsInACall = 2
	cfg.AppendTagsAsAttributes = []string{}
	cfg.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	settings := receivertest.NewNopSettings(metadata.Type)

	optionsResolver := newMockClientOptionsResolver(
		getSubscriptionByIDMockData(),
		getSubscriptionsMockData(),
		getResourcesMockData(),
		getMetricsDefinitionsMockData(),
		nil,
		getMetricsQueryResponseMockData(),
	)

	s := &azureBatchScraper{
		cfg:                          cfg,
		mbs:                          newConcurrentMapImpl[*metadata.MetricsBuilder](),
		mutex:                        &sync.Mutex{},
		time:                         getTimeMock(),
		clientOptionsResolver:        optionsResolver,
		receiverSettings:             settings,
		settings:                     settings.TelemetrySettings,
		storageAccountSpecificConfig: newStorageAccountSpecificConfig(cfg.Services),

		subscriptions: map[string]*azureSubscription{},
		resources:     map[string]map[string]*azureResource{},
		regions:       map[string]map[string]struct{}{},
		resourceTypes: map[string]map[string]*azureType{},
	}

	// First scrape: must produce data.
	first, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.Positive(t, first.DataPointCount(), "first scrape should emit at least one data point")

	// Second scrape immediately after: the temporal guard must prevent
	// re-querying Azure for the same compositeKey, so no new data point
	// should be emitted (which is what avoids the 409 duplicates downstream).
	second, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, second.DataPointCount(),
		"second scrape must not re-emit Azure data points (would cause 409 duplicate sample for timestamp on Prometheus remote write backends)")
}
