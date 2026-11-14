// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import (
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

// TestAzureScraperBatchScrape_ResourcesAreQueriedInTheirOwnRegionOnly checks that a regional batch endpoint is only asked
// about the resources located in that region. The mocked API answers a call only when it holds exactly the resources
// of one region, as a real endpoint answers (with zeros) for resources of other regions, so a call mixing regions must
// not happen: it would not match any mock and no data point would be emitted.
func TestAzureScraperBatchScrape_ResourcesAreQueriedInTheirOwnRegionOnly(t *testing.T) {
	idWestEurope := "/subscriptions/subscriptionId1/resourceGroups/group1/resourceIdWestEurope"
	idEastUS := "/subscriptions/subscriptionId1/resourceGroups/group1/resourceIdEastUS"
	westEurope, eastUS, name1, type1 := "westeurope", "eastus", "name1", "type1"

	resources := newResourcesMockData(map[string][][]*armresources.GenericResourceExpanded{
		"subscriptionId1": {
			{
				{ID: &idWestEurope, Location: &westEurope, Name: &name1, Type: &type1},
				{ID: &idEastUS, Location: &eastUS, Name: &name1, Type: &type1},
			},
		},
	})
	definitions := newMetricsDefinitionMockData(map[string][]metricsDefinitionMockInput{
		idWestEurope: {{namespace: "namespace1", name: "metric1", timeGrain: "PT1M"}},
		idEastUS:     {{namespace: "namespace1", name: "metric1", timeGrain: "PT1M"}},
	})

	ts := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	valueWestEurope, valueEastUS := 11.0, 22.0
	responseFor := func(resourceID string, value *float64) queryResourcesResponseMock {
		return queryResourcesResponseMock{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId1",
				metricNamespace: "namespace1",
				metricNames:     []string{"metric1"},
				resourceIDs:     []string{resourceID},
			},
			response: newQueryResourcesResponseMockData([]queryResourceMockInput{{
				ResourceID: resourceID,
				Metrics: []metricMockInput{{
					Name: "metric1",
					Unit: azmetrics.MetricUnitCount,
					TimeSeries: []azmetrics.TimeSeriesElement{{
						Data: []azmetrics.MetricValue{{TimeStamp: &ts, Average: value}},
					}},
				}},
			}}),
		}
	}

	cfg := createDefaultTestConfig()
	cfg.SubscriptionIDs = []string{"subscriptionId1"}
	cfg.Metrics = NestedListAlias{"namespace1": {"metric1": {"Average"}}}
	settings := receivertest.NewNopSettings(metadata.Type)
	s := &azureBatchScraper{
		cfg:   cfg,
		mbs:   newConcurrentMapImpl[*metadata.MetricsBuilder](),
		mutex: &sync.Mutex{},
		time:  getTimeMock(),
		clientOptionsResolver: newMockClientOptionsResolver(
			getSubscriptionByIDMockData(),
			getSubscriptionsMockData(),
			resources,
			definitions,
			nil,
			[]queryResourcesResponseMock{responseFor(idWestEurope, &valueWestEurope), responseFor(idEastUS, &valueEastUS)},
		),
		receiverSettings:             settings,
		settings:                     settings.TelemetrySettings,
		storageAccountSpecificConfig: newStorageAccountSpecificConfig(cfg.Services),
		subscriptions:                map[string]*azureSubscription{},
		resources:                    map[string]map[string]*azureResource{},
		regions:                      map[string]map[string]struct{}{},
		resourceTypes:                map[string]map[string]*azureType{},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, metrics.DataPointCount(), "one data point for each resource, from the call of its own region")

	valueByResource := map[string]float64{}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		sms := metrics.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				dps := ms.At(k).Gauge().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					id, ok := dps.At(l).Attributes().Get("azuremonitor.resource_id")
					require.True(t, ok)
					valueByResource[id.Str()] = dps.At(l).DoubleValue()
				}
			}
		}
	}
	assert.Equal(t, map[string]float64{idWestEurope: valueWestEurope, idEastUS: valueEastUS}, valueByResource)
}

func TestResourceIDsInRegion(t *testing.T) {
	location := func(l string) *string { return &l }
	s := &azureBatchScraper{resources: map[string]map[string]*azureResource{
		"sub": {
			"id-we-1":     {attributes: map[string]*string{attributeLocation: location("westeurope")}},
			"id-us":       {attributes: map[string]*string{attributeLocation: location("eastus")}},
			"id-we-2":     {attributes: map[string]*string{attributeLocation: location("WestEurope")}},
			"id-global":   {attributes: map[string]*string{}}, // no location: no regional endpoint can be asked about it
			"id-unloaded": nil,
		},
	}}
	all := []string{"id-we-1", "id-us", "id-we-2", "id-global", "id-unloaded", "id-unknown"}

	assert.Equal(t, []string{"id-we-1", "id-we-2"}, s.resourceIDsInRegion("sub", all, "westeurope"), "order is kept and the region is compared case-insensitively")
	assert.Equal(t, []string{"id-us"}, s.resourceIDsInRegion("sub", all, "eastus"))
	assert.Empty(t, s.resourceIDsInRegion("sub", all, "eastasia"))
}
