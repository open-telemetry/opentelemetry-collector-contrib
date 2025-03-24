// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
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

func azIDCredentialsFuncMock(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error) {
	return &azidentity.ClientSecretCredential{}, nil
}

func azIDWorkloadFuncMock(*azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error) {
	return &azidentity.WorkloadIdentityCredential{}, nil
}

func azManagedIdentityFuncMock(*azidentity.ManagedIdentityCredentialOptions) (*azidentity.ManagedIdentityCredential, error) {
	return &azidentity.ManagedIdentityCredential{}, nil
}

func azDefaultCredentialsFuncMock(*azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error) {
	return &azidentity.DefaultAzureCredential{}, nil
}

func createDefaultTestConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.TenantID = "fake-tenant-id"
	return cfg
}

func TestAzureScraperStart(t *testing.T) {
	timeMock := getTimeMock()

	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		// TODO: Add test cases.
		{
			name: "default",
			testFunc: func(t *testing.T) {
				cfg := createDefaultTestConfig()
				s := &azureScraper{
					cfg:                 cfg,
					time:                timeMock,
					azIDCredentialsFunc: azIDCredentialsFuncMock,
					azIDWorkloadFunc:    azIDWorkloadFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.ClientSecretCredential{}, s.cred)
			},
		},
		{
			name: "service_principal",
			testFunc: func(t *testing.T) {
				cfg := createDefaultTestConfig()
				cfg.Authentication = servicePrincipal
				s := &azureScraper{
					cfg:                 cfg,
					time:                timeMock,
					azIDCredentialsFunc: azIDCredentialsFuncMock,
					azIDWorkloadFunc:    azIDWorkloadFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.ClientSecretCredential{}, s.cred)
			},
		},
		{
			name: "workload_identity",
			testFunc: func(t *testing.T) {
				cfg := createDefaultTestConfig()
				cfg.Authentication = workloadIdentity
				s := &azureScraper{
					cfg:                 cfg,
					time:                timeMock,
					azIDCredentialsFunc: azIDCredentialsFuncMock,
					azIDWorkloadFunc:    azIDWorkloadFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.WorkloadIdentityCredential{}, s.cred)
			},
		},
		{
			name: "managed_identity",
			testFunc: func(t *testing.T) {
				cfg := createDefaultTestConfig()
				cfg.Authentication = managedIdentity
				s := &azureScraper{
					cfg:                   cfg,
					time:                  timeMock,
					azIDCredentialsFunc:   azIDCredentialsFuncMock,
					azManagedIdentityFunc: azManagedIdentityFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.ManagedIdentityCredential{}, s.cred)
			},
		},
		{
			name: "default_credentials",
			testFunc: func(t *testing.T) {
				cfg := createDefaultTestConfig()
				cfg.Authentication = defaultCredentials
				s := &azureScraper{
					cfg:                      cfg,
					time:                     timeMock,
					azIDCredentialsFunc:      azIDCredentialsFuncMock,
					azDefaultCredentialsFunc: azDefaultCredentialsFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.DefaultAzureCredential{}, s.cred)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func newMockSubscriptionsListPager(subscriptionsPages []armsubscriptions.ClientListResponse) func(options *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
	return func(_ *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
		for _, page := range subscriptionsPages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}

func newMockResourcesListPager(resourcesPages []armresources.ClientListResponse) func(options *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
	return func(_ *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
		for _, page := range resourcesPages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}

func newMockMetricsDefinitionListPager(metricDefinitionsPagesByResourceURI map[string][]armmonitor.MetricDefinitionsClientListResponse) func(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
	return func(resourceURI string, _ *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		for _, page := range metricDefinitionsPagesByResourceURI[resourceURI] {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}

func newMockMetricList(metricsByResourceURIAndMetricName map[string]map[string]armmonitor.MetricsClientListResponse) func(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
	return func(_ context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		resp.SetResponse(http.StatusOK, metricsByResourceURIAndMetricName[resourceURI][*options.Metricnames], nil)
		return
	}
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
	cfg.SubscriptionIDs = []string{"subscriptionId1", "subscriptionId3"}

	cfgTagsEnabled := createDefaultTestConfig()
	cfgTagsEnabled.AppendTagsAsAttributes = true
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
				ctx: context.Background(),
			},
		},
		{
			name: "metrics_tags_golden",
			fields: fields{
				cfg: cfgTagsEnabled,
			},
			args: args{
				ctx: context.Background(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopSettings(metadata.Type)

			optionsResolver := newMockClientOptionsResolver(
				getSubscriptionsMockData(),
				getResourcesMockData(tt.fields.cfg.AppendTagsAsAttributes),
				getMetricsDefinitionsMockData(),
				getMetricsValuesMockData(),
			)

			s := &azureScraper{
				cfg:                   tt.fields.cfg,
				mb:                    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
				mutex:                 &sync.Mutex{},
				time:                  getTimeMock(),
				clientOptionsResolver: optionsResolver,

				// From there, initialize everything that is normally initialized in start() func
				subscriptions: map[string]*azureSubscription{
					"subscriptionId1": {SubscriptionID: "subscriptionId1"},
					"subscriptionId3": {SubscriptionID: "subscriptionId3"},
				},
				resources: map[string]map[string]*azureResource{
					"subscriptionId1": {},
					"subscriptionId3": {},
				},
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
	metricName1, metricName2, metricName3 := "ConnectionsTotal", "IncommingMessages", "TransferedBytes"
	metricAggregation1, metricAggregation2, metricAggregation3 := "Count", "Maximum", "Minimum"
	cfgLimitedMertics := createDefaultTestConfig()
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

		resourceMockData := map[string][]armresources.ClientListResponse{
			fakeSubID: {
				{
					ResourceListResult: armresources.ResourceListResult{
						Value: []*armresources.GenericResourceExpanded{
							{
								ID:       &id1,
								Location: &location,
								Name:     &name,
								Type:     &metricNamespace1,
							},
						},
					},
				},
				{
					ResourceListResult: armresources.ResourceListResult{
						Value: []*armresources.GenericResourceExpanded{
							{
								ID:       &id2,
								Location: &location,
								Name:     &name,
								Type:     &metricNamespace2,
							},
						},
					},
				},
			},
		}

		metricsDefinitionMockData := map[string][]armmonitor.MetricDefinitionsClientListResponse{
			id1: {
				{
					MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
						Value: []*armmonitor.MetricDefinition{
							{
								Namespace: &metricNamespace1,
								Name: &armmonitor.LocalizableString{
									Value: &metricName1,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
						},
					},
				},
			},
			id2: {
				{
					MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
						Value: []*armmonitor.MetricDefinition{
							{
								Namespace: &metricNamespace2,
								Name: &armmonitor.LocalizableString{
									Value: &metricName2,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
							{
								Namespace: &metricNamespace2,
								Name: &armmonitor.LocalizableString{
									Value: &metricName3,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
						},
					},
				},
			},
		}

		metricsMockData := map[string]map[string]armmonitor.MetricsClientListResponse{
			id1: {
				metricName1: {
					Response: armmonitor.Response{
						Value: []*armmonitor.Metric{
							{
								Name: &armmonitor.LocalizableString{
									Value: &metricName1,
								},
								Unit: &unit,
								Timeseries: []*armmonitor.TimeSeriesElement{
									{
										Data: []*armmonitor.MetricValue{
											{
												Count: &valueCount,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			id2: {
				metricName2: {
					Response: armmonitor.Response{
						Value: []*armmonitor.Metric{
							{
								Name: &armmonitor.LocalizableString{
									Value: &metricName2,
								},
								Unit: &unit,
								Timeseries: []*armmonitor.TimeSeriesElement{
									{
										Data: []*armmonitor.MetricValue{
											{
												Average: &valueMaximum,
												Count:   &valueCount,
												Maximum: &valueMaximum,
												Minimum: &valueMinimum,
												Total:   &valueCount,
											},
										},
									},
								},
							},
						},
					},
				},
				metricName3: {
					Response: armmonitor.Response{
						Value: []*armmonitor.Metric{
							{
								Name: &armmonitor.LocalizableString{
									Value: &metricName3,
								},
								Unit: &unit,
								Timeseries: []*armmonitor.TimeSeriesElement{
									{
										Data: []*armmonitor.MetricValue{
											{
												Maximum: &valueMaximum,
												Minimum: &valueMinimum,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		optionsResolver := newMockClientOptionsResolver(
			getSubscriptionsMockData(),
			resourceMockData,
			metricsDefinitionMockData,
			metricsMockData,
		)

		settings := receivertest.NewNopSettings(metadata.Type)
		s := &azureScraper{
			cfg:                   cfgLimitedMertics,
			mb:                    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
			mutex:                 &sync.Mutex{},
			time:                  getTimeMock(),
			clientOptionsResolver: optionsResolver,

			// From there, initialize everything that is normally initialized in start() func
			subscriptions: map[string]*azureSubscription{
				fakeSubID: {SubscriptionID: fakeSubID},
			},
			resources: map[string]map[string]*azureResource{
				fakeSubID: {},
			},
		}

		metrics, err := s.scrape(context.Background())

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

func getSubscriptionsMockData() []armsubscriptions.ClientListResponse {
	return []armsubscriptions.ClientListResponse{
		{
			SubscriptionListResult: armsubscriptions.SubscriptionListResult{
				Value: []*armsubscriptions.Subscription{
					{ID: to.Ptr("subscriptionId1")},
					{ID: to.Ptr("subscriptionId2")},
				},
			},
		},
		{
			SubscriptionListResult: armsubscriptions.SubscriptionListResult{
				Value: []*armsubscriptions.Subscription{
					{ID: to.Ptr("subscriptionId3")},
				},
			},
		},
	}
}

func TestAzureScraperScrapeHonorTimeGrain(t *testing.T) {
	getTestScraper := func() *azureScraper {
		optionsResolver := newMockClientOptionsResolver(
			getSubscriptionsMockData(),
			getResourcesMockData(false),
			getMetricsDefinitionsMockData(),
			getMetricsValuesMockData(),
		)

		return &azureScraper{
			cfg: createDefaultTestConfig(),
			mb: metadata.NewMetricsBuilder(
				metadata.DefaultMetricsBuilderConfig(),
				receivertest.NewNopSettings(receivertest.NopType),
			),
			mutex:                 &sync.Mutex{},
			time:                  getTimeMock(),
			clientOptionsResolver: optionsResolver,

			// From there, initialize everything that is normally initialized in start() func
			subscriptions: map[string]*azureSubscription{
				"subscriptionId1": {SubscriptionID: "subscriptionId1"},
			},
			resources: map[string]map[string]*azureResource{
				"subscriptionId1": {},
			},
		}
	}

	ctx := context.Background()

	t.Run("do_not_fetch_in_same_interval", func(t *testing.T) {
		s := getTestScraper()

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
		s := getTestScraper()
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

type timeMock struct {
	time time.Time
}

func (t *timeMock) Now() time.Time {
	return t.time
}

func getTimeMock() timeNowIface {
	return &timeMock{time: time.Now()}
}

func getResourcesMockData(tags bool) map[string][]armresources.ClientListResponse {
	id1, id2, id3, id4,
		location1, name1, type1 := "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1",
		"location1", "name1", "type1"

	resourceID1 := armresources.GenericResourceExpanded{
		ID:       &id1,
		Location: &location1,
		Name:     &name1,
		Type:     &type1,
	}
	if tags {
		tagName1, tagValue1 := "tagName1", "tagValue1"
		resourceID1.Tags = map[string]*string{tagName1: &tagValue1}
	}
	return map[string][]armresources.ClientListResponse{
		"subscriptionId1": {
			{
				ResourceListResult: armresources.ResourceListResult{
					Value: []*armresources.GenericResourceExpanded{
						&resourceID1,
						{
							ID:       &id2,
							Location: &location1,
							Name:     &name1,
							Type:     &type1,
						},
					},
				},
			},
			{
				ResourceListResult: armresources.ResourceListResult{
					Value: []*armresources.GenericResourceExpanded{
						{
							ID:       &id3,
							Location: &location1,
							Name:     &name1,
							Type:     &type1,
						},
					},
				},
			},
		},
		"subscriptionId3": {{
			ResourceListResult: armresources.ResourceListResult{
				Value: []*armresources.GenericResourceExpanded{
					{
						ID:       &id4,
						Location: &location1,
						Name:     &name1,
						Type:     &type1,
					},
				},
			},
		}},
	}
}

func getMetricsDefinitionsMockData() map[string][]armmonitor.MetricDefinitionsClientListResponse {
	namespace1, namespace2, name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension1, dimension2 := "namespace1",
		"namespace2", "metric1", "metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension1", "dimension2"

	return map[string][]armmonitor.MetricDefinitionsClientListResponse{
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1": {
			{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name1,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
						},
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name2,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
						},
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name3,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2": {
			{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name4,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
						},
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name5,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain2,
								},
							},
							Dimensions: []*armmonitor.LocalizableString{
								{
									Value: &dimension1,
								},
								{
									Value: &dimension2,
								},
							},
						},
						{
							Namespace: &namespace1,
							Name: &armmonitor.LocalizableString{
								Value: &name6,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain2,
								},
							},
							Dimensions: []*armmonitor.LocalizableString{
								{
									Value: &dimension1,
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3": {
			{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						{
							Namespace: &namespace2,
							Name: &armmonitor.LocalizableString{
								Value: &name7,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
							Dimensions: []*armmonitor.LocalizableString{
								{
									Value: &dimension1,
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1": {
			{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						{
							Namespace: &namespace2,
							Name: &armmonitor.LocalizableString{
								Value: &name7,
							},
							MetricAvailabilities: []*armmonitor.MetricAvailability{
								{
									TimeGrain: &timeGrain1,
								},
							},
							Dimensions: []*armmonitor.LocalizableString{
								{
									Value: &dimension1,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getMetricsValuesMockData() map[string]map[string]armmonitor.MetricsClientListResponse {
	name1, name2, name3, name4, name5, name6, name7, dimension1, dimension2, dimensionValue := "metric1", "metric2",
		"metric3", "metric4", "metric5", "metric6", "metric7", "dimension1", "dimension2", "dimension value"
	var unit1 armmonitor.Unit = "unit1"
	var value1 float64 = 1

	return map[string]map[string]armmonitor.MetricsClientListResponse{
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1": {
			strings.Join([]string{name1, name2}, ","): {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name1,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
								},
							},
						},
						{
							Name: &armmonitor.LocalizableString{
								Value: &name2,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
								},
							},
						},
					},
				},
			},
			name3: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name3,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2": {
			name4: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name4,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
								},
							},
						},
					},
				},
			},
			name5: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name5,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
									Metadatavalues: []*armmonitor.MetadataValue{
										{
											Name: &armmonitor.LocalizableString{
												Value: &dimension1,
											},
											Value: &dimensionValue,
										},
										{
											Name: &armmonitor.LocalizableString{
												Value: &dimension2,
											},
											Value: &dimensionValue,
										},
									},
								},
							},
						},
					},
				},
			},
			name6: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name6,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Average: &value1,
											Count:   &value1,
											Maximum: &value1,
											Minimum: &value1,
											Total:   &value1,
										},
									},
									Metadatavalues: []*armmonitor.MetadataValue{
										{
											Name: &armmonitor.LocalizableString{
												Value: &dimension1,
											},
											Value: &dimensionValue,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3": {
			name7: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name7,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Count: &value1,
										},
									},
									Metadatavalues: []*armmonitor.MetadataValue{
										{
											Name: &armmonitor.LocalizableString{
												Value: &dimension1,
											},
											Value: &dimensionValue,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1": {
			name7: {
				Response: armmonitor.Response{
					Value: []*armmonitor.Metric{
						{
							Name: &armmonitor.LocalizableString{
								Value: &name7,
							},
							Unit: &unit1,
							Timeseries: []*armmonitor.TimeSeriesElement{
								{
									Data: []*armmonitor.MetricValue{
										{
											Count: &value1,
										},
									},
									Metadatavalues: []*armmonitor.MetadataValue{
										{
											Name: &armmonitor.LocalizableString{
												Value: &dimension1,
											},
											Value: &dimensionValue,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
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
