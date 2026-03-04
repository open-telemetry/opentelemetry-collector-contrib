// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

func TestNewScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	require.Empty(t, scraper.resources)
}

func createDefaultTestConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.TenantID = "fake-tenant-id"
	cfg.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}
	return cfg
}

func TestAzureScraperScrape(t *testing.T) {
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

	cfgTagsEnabled := createDefaultTestConfig()
	cfgTagsEnabled.AppendTagsAsAttributes = []string{"*"}
	cfgTagsEnabled.MaximumNumberOfMetricsInACall = 2
	cfgTagsEnabled.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgTagsSelective := createDefaultTestConfig()
	cfgTagsSelective.AppendTagsAsAttributes = []string{"tagName1"}
	cfgTagsSelective.MaximumNumberOfMetricsInACall = 2
	cfgTagsSelective.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgSubNameAttr := createDefaultTestConfig()
	cfgSubNameAttr.AppendTagsAsAttributes = []string{}
	cfgSubNameAttr.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}
	cfgSubNameAttr.MetricsBuilderConfig.ResourceAttributes.AzuremonitorSubscription.Enabled = true

	cfgTagsCaseInsensitive := createDefaultTestConfig()
	cfgTagsCaseInsensitive.AppendTagsAsAttributes = []string{"TAGNAME1"}
	cfgTagsCaseInsensitive.MaximumNumberOfMetricsInACall = 2
	cfgTagsCaseInsensitive.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

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
			name: "metrics_subname_golden",
			fields: fields{
				cfg: cfgSubNameAttr,
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
				getMetricsValuesMockData(),
				nil,
			)

			s := &azureScraper{
				cfg:                   tt.fields.cfg,
				mb:                    metadata.NewMetricsBuilder(tt.fields.cfg.MetricsBuilderConfig, settings),
				mutex:                 &sync.Mutex{},
				time:                  getTimeMock(),
				clientOptionsResolver: optionsResolver,

				// From there, initialize everything that is normally initialized in start() func
				subscriptions: map[string]*azureSubscription{},
				resources:     map[string]map[string]*azureResource{},
			}

			metrics, err := s.scrape(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			expectedFile := filepath.Join("testdata", "expected_metrics", tt.name+".yaml")
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

func TestAzureScraperScrapeFilterMetrics(t *testing.T) {
	fakeSubID := "azuremonitor-receiver"
	metricNamespace1, metricNamespace2 := "Microsoft.ServiceA/namespace1", "Microsoft.ServiceB/namespace2"
	metricName1, metricName2, metricName3 := "ConnectionsTotal", "IncomingMessages", "TransferredBytes"
	metricAggregation1, metricAggregation2, metricAggregation3 := "Count", "Maximum", "Minimum"
	cfgLimitedMertics := createDefaultTestConfig()
	cfgLimitedMertics.SubscriptionIDs = []string{fakeSubID}
	cfgLimitedMertics.Metrics = NestedListAlias{
		metricNamespace1: {
			metricName1: {metricAggregation1},
		},
		metricNamespace2: {
			metricName2: {filterAllAggregations},
			metricName3: {metricAggregation2, metricAggregation3},
		},
	}

	t.Run("should filter metrics and aggregations", func(t *testing.T) {
		name := "resource-name"
		id1 := "/subscription/" + fakeSubID + "/resourceGroups/resource-group/providers/" + metricNamespace1 + "/" + name
		id2 := "/subscription/" + fakeSubID + "/resourceGroups/resource-group/providers/" + metricNamespace2 + "/" + name
		location := "location-name"
		timeGrain := "PT1M"
		var unit armmonitor.Unit = "u"
		var valueCount float64 = 11
		valueMaximum := 123.45
		valueMinimum := 0.1

		subscriptionsByIDMockData := newSubscriptionsByIDMockData(map[string]string{
			fakeSubID: "displayname",
		})
		resourceMockData := newResourcesMockData(map[string][][]*armresources.GenericResourceExpanded{
			fakeSubID: {
				{{ID: &id1, Location: &location, Name: &name, Type: &metricNamespace1}},
				{{ID: &id2, Location: &location, Name: &name, Type: &metricNamespace2}},
			},
		})

		metricsDefinitionMockData := newMetricsDefinitionMockData(map[string][]metricsDefinitionMockInput{
			id1: {
				{namespace: metricNamespace1, name: metricName1, timeGrain: timeGrain},
			},
			id2: {
				{namespace: metricNamespace2, name: metricName2, timeGrain: timeGrain},
				{namespace: metricNamespace2, name: metricName3, timeGrain: timeGrain},
			},
		})

		metricsMockData := newMetricsClientListResponseMockData(map[string]map[string][]metricsClientListResponseMockInput{
			id1: {
				metricName1: {{
					Name: metricName1,
					Unit: unit,
					TimeSeries: []*armmonitor.TimeSeriesElement{{Data: []*armmonitor.MetricValue{
						{Count: &valueCount},
					}}},
				}},
			},
			id2: {
				metricName2: {{
					Name: metricName2,
					Unit: unit,
					TimeSeries: []*armmonitor.TimeSeriesElement{{Data: []*armmonitor.MetricValue{
						{Average: &valueMaximum, Count: &valueCount, Maximum: &valueMaximum, Minimum: &valueMinimum, Total: &valueCount},
					}}},
				}},
				metricName3: {{
					Name: metricName3,
					Unit: unit,
					TimeSeries: []*armmonitor.TimeSeriesElement{{Data: []*armmonitor.MetricValue{
						{Maximum: &valueMaximum, Minimum: &valueMinimum},
					}}},
				}},
			},
		})

		optionsResolver := newMockClientOptionsResolver(
			subscriptionsByIDMockData,
			getSubscriptionsMockData(),
			resourceMockData,
			metricsDefinitionMockData,
			metricsMockData,
			nil,
		)

		settings := receivertest.NewNopSettings(metadata.Type)
		s := &azureScraper{
			cfg:                   cfgLimitedMertics,
			mb:                    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
			mutex:                 &sync.Mutex{},
			time:                  getTimeMock(),
			clientOptionsResolver: optionsResolver,

			// From there, initialize everything that is normally initialized in start() func
			subscriptions: map[string]*azureSubscription{},
			resources:     map[string]map[string]*azureResource{},
		}

		metrics, err := s.scrape(t.Context())

		require.NoError(t, err)
		expectedFile := filepath.Join("testdata", "expected_metrics", "metrics_filtered.yaml")
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

func getSubscriptionByIDMockData() map[string]armsubscriptions.ClientGetResponse {
	return newSubscriptionsByIDMockData(map[string]string{
		"subscriptionId1": "subscriptionDisplayName1",
		"subscriptionId2": "subscriptionDisplayName2",
		"subscriptionId3": "subscriptionDisplayName3",
	})
}

func getSubscriptionsMockData() []armsubscriptions.ClientListResponse {
	return newSubscriptionsListMockData([][]string{
		{"subscriptionId1", "subscriptionId2"},
		{"subscriptionId3"},
	})
}

func getNominalTestScraper() *azureScraper {
	optionsResolver := newMockClientOptionsResolver(
		getSubscriptionByIDMockData(),
		getSubscriptionsMockData(),
		getResourcesMockData(),
		getMetricsDefinitionsMockData(),
		getMetricsValuesMockData(),
		nil,
	)

	settings := receivertest.NewNopSettings(metadata.Type)

	return &azureScraper{
		cfg:                   createDefaultTestConfig(),
		settings:              settings.TelemetrySettings,
		mb:                    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		mutex:                 &sync.Mutex{},
		time:                  getTimeMock(),
		clientOptionsResolver: optionsResolver,

		// From there, initialize everything that is normally initialized in start() func
		subscriptions: map[string]*azureSubscription{},
		resources:     map[string]map[string]*azureResource{},
	}
}

func TestAzureScraperGetResources(t *testing.T) {
	s := getNominalTestScraper()
	s.resources["subscriptionId1"] = map[string]*azureResource{}
	s.subscriptions["subscriptionId1"] = &azureSubscription{}
	s.cfg.CacheResources = 0
	s.getResources(t.Context(), "subscriptionId1")
	assert.Contains(t, s.resources, "subscriptionId1")
	assert.Len(t, s.resources["subscriptionId1"], 3)

	s.clientOptionsResolver = newMockClientOptionsResolver(
		getSubscriptionByIDMockData(),
		getSubscriptionsMockData(),
		map[string][]armresources.ClientListResponse{
			"subscriptionId1": {{
				ResourceListResult: armresources.ResourceListResult{
					Value: nil, // Simulate resources disappear
				},
			}},
		},
		getMetricsDefinitionsMockData(),
		getMetricsValuesMockData(),
		nil,
	)
	s.getResources(t.Context(), "subscriptionId1")
	assert.Contains(t, s.resources, "subscriptionId1")
	assert.Empty(t, s.resources["subscriptionId1"])
}

func TestAzureScraperScrapeHonorTimeGrain(t *testing.T) {
	ctx := t.Context()

	t.Run("do_not_fetch_in_same_interval", func(t *testing.T) {
		s := getNominalTestScraper()

		metrics, err := s.scrape(ctx)

		require.NoError(t, err, "should not fail")
		require.Positive(t, metrics.MetricCount(), "should return metrics on first call")

		metrics, err = s.scrape(ctx)

		require.NoError(t, err, "should not fail")
		require.Equal(t, 0, metrics.MetricCount(), "should not return metrics on second call")
	})

	t.Run("fetch_each_new_interval", func(t *testing.T) {
		timeJitter := time.Second
		timeInterval := 50 * time.Second
		timeIntervals := []time.Time{
			time.Now().Add(time.Minute),
			time.Now().Add(time.Minute + 1*timeInterval - timeJitter),
			time.Now().Add(time.Minute + 2*timeInterval + timeJitter),
			time.Now().Add(time.Minute + 3*timeInterval - timeJitter),
			time.Now().Add(time.Minute + 4*timeInterval + timeJitter),
		}
		s := getNominalTestScraper()
		mockedTime := s.time.(*timeMock)

		for _, timeNowNew := range timeIntervals {
			// implementation uses time.Sub to check if timeWrapper.Now satisfy time grain
			// we can travel in time by adjusting result of timeWrapper.Now
			prevTime := mockedTime.time
			mockedTime.time = timeNowNew

			metrics, err := s.scrape(ctx)

			require.NoError(t, err, "should not fail")
			if prevTime.Minute() == timeNowNew.Minute() {
				require.Equal(t, 0, metrics.MetricCount(), "should not fetch metrics in the same minute")
			} else {
				require.Positive(t, metrics.MetricCount(), "should fetch metrics in a new minute")
			}
		}
	})
}

func getTimeMock() timeNowIface {
	return &timeMock{time: time.Now()}
}

func getResourcesMockData() map[string][]armresources.ClientListResponse {
	id1, id2, id3, id4,
		location1, name1, type1 := "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1",
		"location1", "name1", "type1"

	tagName1, tagValue1 := "tagName1", "tagValue1"
	tagName2, tagValue2 := "tagName2", "tagValue2"

	return newResourcesMockData(map[string][][]*armresources.GenericResourceExpanded{
		"subscriptionId1": {
			{
				{
					ID: &id1, Location: &location1, Name: &name1, Type: &type1,
					Tags: map[string]*string{
						tagName1: &tagValue1,
						tagName2: &tagValue2,
					},
				},
				{
					ID: &id2, Location: &location1, Name: &name1, Type: &type1,
				},
			},
			{
				{
					ID: &id3, Location: &location1, Name: &name1, Type: &type1,
				},
			},
		},
		"subscriptionId3": {
			{
				{
					ID: &id4, Location: &location1, Name: &name1, Type: &type1,
				},
			},
		},
	})
}

func getMetricsDefinitionsMockData() map[string][]armmonitor.MetricDefinitionsClientListResponse {
	namespace1, namespace2, name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension1, dimension2 := "namespace1",
		"namespace2", "metric1", "metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension1", "dimension2"

	return newMetricsDefinitionMockData(map[string][]metricsDefinitionMockInput{
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1": {
			{namespace: namespace1, name: name1, timeGrain: timeGrain1},
			{namespace: namespace1, name: name2, timeGrain: timeGrain1},
			{namespace: namespace1, name: name3, timeGrain: timeGrain1},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2": {
			{namespace: namespace1, name: name4, timeGrain: timeGrain1},
			{namespace: namespace1, name: name5, timeGrain: timeGrain2, dimensions: []string{dimension1, dimension2}},
			{namespace: namespace1, name: name6, timeGrain: timeGrain2, dimensions: []string{dimension1}},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3": {
			{namespace: namespace2, name: name7, timeGrain: timeGrain1},
		},
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1": {
			{namespace: namespace2, name: name7, timeGrain: timeGrain1},
		},
	})
}

func getMetricsValuesMockData() map[string]map[string]armmonitor.MetricsClientListResponse {
	name1, name2, name3, name4, name5, name6, name7, dimension1, dimension2, dimensionValue := "metric1", "metric2",
		"metric3", "metric4", "metric5", "metric6", "metric7", "dimension1", "dimension2", "dimension value"
	var unit1 armmonitor.Unit = "unit1"
	var value1 float64 = 1

	return newMetricsClientListResponseMockData(map[string]map[string][]metricsClientListResponseMockInput{
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1": {
			name1 + "," + name2: {
				{
					Name: name1, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
					}},
				},
				{
					Name: name2, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
					}},
				},
			},
			name3: {
				{
					Name: name3, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
					}},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2": {
			name4: {
				{
					Name: name4, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
					}},
				},
			},
			name5: {
				{
					Name: name5, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
						Metadatavalues: []*armmonitor.MetadataValue{
							{Name: &armmonitor.LocalizableString{Value: &dimension1}, Value: &dimensionValue},
							{Name: &armmonitor.LocalizableString{Value: &dimension2}, Value: &dimensionValue},
						},
					}},
				},
			},
			name6: {
				{
					Name: name6, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Average: &value1, Count: &value1, Maximum: &value1, Minimum: &value1, Total: &value1},
						},
						Metadatavalues: []*armmonitor.MetadataValue{
							{Name: &armmonitor.LocalizableString{Value: &dimension1}, Value: &dimensionValue},
						},
					}},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3": {
			name7: {
				{
					Name: name7, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Count: &value1},
						},
						Metadatavalues: []*armmonitor.MetadataValue{
							{Name: &armmonitor.LocalizableString{Value: &dimension1}, Value: &dimensionValue},
						},
					}},
				},
			},
		},
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1": {
			name7: {
				{
					Name: name7, Unit: unit1,
					TimeSeries: []*armmonitor.TimeSeriesElement{{
						Data: []*armmonitor.MetricValue{
							{Count: &value1},
						},
						Metadatavalues: []*armmonitor.MetadataValue{
							{Name: &armmonitor.LocalizableString{Value: &dimension1}, Value: &dimensionValue},
						},
					}},
				},
			},
		},
	})
}

func TestGetMetricAggregations(t *testing.T) {
	testNamespaceName := "Microsoft.AAD/DomainServices"
	testMetricName := "MetricName"
	tests := []struct {
		name    string
		filters NestedListAlias
		want    []string
	}{
		{
			name:    "should return all aggregations when metrics filter empty",
			filters: NestedListAlias{},
			want:    aggregations,
		},
		{
			name: "should return all aggregations when namespace not in filters",
			filters: NestedListAlias{
				"another.namespace": nil,
			},
			want: aggregations,
		},
		{
			name: "should return all aggregations when metric in filters",
			filters: NestedListAlias{
				testNamespaceName: {
					testMetricName: {},
				},
			},
			want: aggregations,
		},
		{
			name: "should return all aggregations ignoring metric name case",
			filters: NestedListAlias{
				testNamespaceName: {
					strings.ToLower(testMetricName): {},
				},
			},
			want: aggregations,
		},
		{
			name: "should return all aggregations when asterisk in filters",
			filters: NestedListAlias{
				testNamespaceName: {
					testMetricName: {filterAllAggregations},
				},
			},
			want: aggregations,
		},
		{
			name: "should be empty when metric not in filters",
			filters: NestedListAlias{
				testNamespaceName: {
					"not_this_metric": {},
				},
			},
			want: []string{},
		},
		{
			name: "should return one aggregations",
			filters: NestedListAlias{
				testNamespaceName: {
					testMetricName: {aggregations[0]},
				},
			},
			want: []string{aggregations[0]},
		},
		{
			name: "should return one aggregations ignoring aggregation case",
			filters: NestedListAlias{
				testNamespaceName: {
					testMetricName: {strings.ToLower(aggregations[0])},
				},
			},
			want: []string{aggregations[0]},
		},
		{
			name: "should return many aggregations",
			filters: NestedListAlias{
				testNamespaceName: {
					testMetricName: {aggregations[0], aggregations[2]},
				},
			},
			want: []string{aggregations[0], aggregations[2]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMetricAggregations(testNamespaceName, testMetricName, tt.filters)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMapFindInsensitive(t *testing.T) {
	testNamespace := "Microsoft.AAD/DomainServices"
	testStr := "should be fine"
	testFilters := map[string]string{
		"microsoft.insights/components": "text",
		testNamespace:                   testStr,
	}
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "should find when same case",
			key:  testNamespace,
			want: true,
		},
		{
			name: "should find when different case",
			key:  strings.ToLower(testNamespace),
			want: true,
		},
		{
			name: "should not find when not exists",
			key:  "microsoft.eventhub/namespaces",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mapFindInsensitive(testFilters, tt.key)
			require.Equal(t, tt.want, ok)
			if ok {
				require.Equal(t, testStr, got)
			}
		})
	}
}
