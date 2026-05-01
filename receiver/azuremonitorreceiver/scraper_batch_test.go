// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
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

				// From there, initialize everything normally initialized in start() func
				subscriptions: newUpdatedMap[string, *azureSubscription](),
				resourceTypes: azResourceTypeStore{},
				resources:     azResourceStore{},
				regions:       azRegionStore{},
				metrics:       azMetricsStore{},
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

// TestAzureScraperBatchScrape_NoRaceWithManyResourceTypes ensures that the producer/consumer
// pipeline in (*azureBatchScraper).scrape() does not race on the s.metrics[subscriptionID] map.
//
// The producer (loadResourceMetricsDefinitionsByType) and the consumers (loadBatchMetricsValues)
// run concurrently and both touch s.metrics[subscriptionID]. Go maps are NOT safe for concurrent
// read/write even on different keys: a write may trigger a rehash that invalidates concurrent
// reads. To trigger this reliably, this test fans out across several distinct resourceTypes per
// subscription so the producer keeps writing new entries while consumers iterate over previously
// pushed ones, and runs scrape() many times to widen the race window.
//
// Run with `-race` to detect regressions: any future change that re-introduces a write to the
// outer s.metrics[subscriptionID] map after the goroutines have started will fail this test.
func TestAzureScraperBatchScrape_NoRaceWithManyResourceTypes(t *testing.T) {
	const (
		sub1     = "subRace1"
		sub2     = "subRace2"
		typeA    = "Microsoft.Race/typeA"
		typeB    = "Microsoft.Race/typeB"
		typeC    = "Microsoft.Race/typeC"
		location = "westeurope"
	)
	id := func(sub, rt string) string {
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/rg/providers/%s/r", sub, rt)
	}

	cfg := createDefaultTestConfig()
	cfg.SubscriptionIDs = []string{sub1, sub2}
	cfg.AppendTagsAsAttributes = []string{}
	cfg.MaximumNumberOfMetricsInACall = 2
	cfg.Services = []string{typeA, typeB, typeC}

	// One resource per (subscription, type). Multiple types per subscription is what
	// triggers the race: the producer keeps inserting new keys in s.metrics[sub] while
	// consumers iterate previously pushed ones.
	resources := map[string][][]*armresources.GenericResourceExpanded{
		sub1: {{
			{ID: to.Ptr(id(sub1, typeA)), Location: to.Ptr(location), Name: to.Ptr("rA"), Type: to.Ptr(typeA)},
			{ID: to.Ptr(id(sub1, typeB)), Location: to.Ptr(location), Name: to.Ptr("rB"), Type: to.Ptr(typeB)},
			{ID: to.Ptr(id(sub1, typeC)), Location: to.Ptr(location), Name: to.Ptr("rC"), Type: to.Ptr(typeC)},
		}},
		sub2: {{
			{ID: to.Ptr(id(sub2, typeA)), Location: to.Ptr(location), Name: to.Ptr("rA"), Type: to.Ptr(typeA)},
			{ID: to.Ptr(id(sub2, typeB)), Location: to.Ptr(location), Name: to.Ptr("rB"), Type: to.Ptr(typeB)},
			{ID: to.Ptr(id(sub2, typeC)), Location: to.Ptr(location), Name: to.Ptr("rC"), Type: to.Ptr(typeC)},
		}},
	}

	// One trivial metric definition per resource so the scraper produces actual composite
	// keys and therefore exercises loadBatchMetricsValues -> reads on s.metrics[sub][type].
	metricsDefs := map[string][]metricsDefinitionMockInput{
		id(sub1, typeA): {{namespace: typeA, name: "raceMetric", timeGrain: "PT1M"}},
		id(sub1, typeB): {{namespace: typeB, name: "raceMetric", timeGrain: "PT1M"}},
		id(sub1, typeC): {{namespace: typeC, name: "raceMetric", timeGrain: "PT1M"}},
		id(sub2, typeA): {{namespace: typeA, name: "raceMetric", timeGrain: "PT1M"}},
		id(sub2, typeB): {{namespace: typeB, name: "raceMetric", timeGrain: "PT1M"}},
		id(sub2, typeC): {{namespace: typeC, name: "raceMetric", timeGrain: "PT1M"}},
	}

	// Empty-but-valid query response per (subscription, namespace). Values are irrelevant:
	// the test only cares that consumers actually read s.metrics[sub][type].
	mkQuery := func(sub, rt string) queryResourcesResponseMock {
		return queryResourcesResponseMock{
			params: queryResourcesResponseMockParams{
				subscriptionID:  sub,
				metricNamespace: rt,
				metricNames:     []string{"raceMetric"},
				resourceIDs:     []string{id(sub, rt)},
			},
			response: newQueryResourcesResponseMockData(nil),
		}
	}
	queryMocks := []queryResourcesResponseMock{
		mkQuery(sub1, typeA), mkQuery(sub1, typeB), mkQuery(sub1, typeC),
		mkQuery(sub2, typeA), mkQuery(sub2, typeB), mkQuery(sub2, typeC),
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	optionsResolver := newMockClientOptionsResolver(
		newSubscriptionsByIDMockData(map[string]string{sub1: sub1 + "-display", sub2: sub2 + "-display"}),
		nil,
		newResourcesMockData(resources),
		newMetricsDefinitionMockData(metricsDefs),
		nil,
		queryMocks,
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

		subscriptions: newUpdatedMap[string, *azureSubscription](),
		resourceTypes: azResourceTypeStore{},
		resources:     azResourceStore{},
		regions:       azRegionStore{},
		metrics:       azMetricsStore{},
	}

	// We don't compare against a golden file: the value of this test is the race detector.
	// Run several iterations to widen the race window (compensates for the small number
	// of resource types).
	for range 20 {
		_, err := s.scrape(t.Context())
		require.NoError(t, err)
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
