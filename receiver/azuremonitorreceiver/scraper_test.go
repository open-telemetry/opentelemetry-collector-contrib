// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	armmonitorfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armresourcesfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
	armsubscriptionsfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions/fake"
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

	scraper := newScraper(cfg, receivertest.NewNopSettings())
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

func TestAzureScraperStart(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		// TODO: Add test cases.
		{
			name: "default",
			testFunc: func(t *testing.T) {
				s := &azureScraper{
					cfg:                 cfg,
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
				customCfg := &Config{
					ControllerConfig:              cfg.ControllerConfig,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                servicePrincipal,
				}
				s := &azureScraper{
					cfg:                 customCfg,
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
				customCfg := &Config{
					ControllerConfig:              cfg.ControllerConfig,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                workloadIdentity,
				}
				s := &azureScraper{
					cfg:                 customCfg,
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
				customCfg := &Config{
					ControllerConfig:              cfg.ControllerConfig,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                managedIdentity,
				}
				s := &azureScraper{
					cfg:                   customCfg,
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
				customCfg := &Config{
					ControllerConfig:              cfg.ControllerConfig,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                defaultCredentials,
				}
				s := &azureScraper{
					cfg:                      customCfg,
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

func newMockSubscriptionsListPager(pages []armsubscriptions.ClientListResponse) func(options *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
	return func(options *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
		for _, page := range pages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}

func newMockResourcesListPager(pages []armresources.ClientListResponse) func(options *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
	return func(options *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
		for _, page := range pages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}

func newMockMetricsDefinitionListPager(pagesByResourceURI map[string][]armmonitor.MetricDefinitionsClientListResponse) func(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
	return func(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		for _, page := range pagesByResourceURI[resourceURI] {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return
	}
}
func newMockMetricList(listsByResourceURIAndMetricName map[string]map[string]armmonitor.MetricsClientListResponse) func(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
	return func(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		resp.SetResponse(http.StatusOK, listsByResourceURIAndMetricName[resourceURI][*options.Metricnames], nil)
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
	cfg := createDefaultConfig().(*Config)
	cfg.MaximumNumberOfMetricsInACall = 2
	cfg.SubscriptionIDs = []string{"subscriptionId1"}

	cfgTagsEnabled := createDefaultConfig().(*Config)
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
			settings := receivertest.NewNopSettings()

			resourceServer := armresourcesfake.Server{
				NewListPager: newMockResourcesListPager(getResourcesMockData(tt.fields.cfg.AppendTagsAsAttributes)),
			}
			subscriptionsServer := armsubscriptionsfake.Server{
				NewListPager: newMockSubscriptionsListPager(getSubscriptionsMockData()),
			}
			monitorServerFactory := armmonitorfake.ServerFactory{
				MetricDefinitionsServer: armmonitorfake.MetricDefinitionsServer{
					NewListPager: newMockMetricsDefinitionListPager(getMetricsDefinitionsMockData()),
				},
				MetricsServer: armmonitorfake.MetricsServer{
					List: newMockMetricList(getMetricsValuesMockData()),
				},
			}

			s := &azureScraper{
				cfg:   tt.fields.cfg,
				mb:    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
				mutex: &sync.Mutex{},
				// From there, initialize everything that is normally initialized in start() func
				armResourcesClientOptions: &arm.ClientOptions{
					ClientOptions: azcore.ClientOptions{
						Transport: armresourcesfake.NewServerTransport(&resourceServer),
					},
				},
				armSubscriptionsClientOptions: &arm.ClientOptions{
					ClientOptions: azcore.ClientOptions{
						Transport: armsubscriptionsfake.NewServerTransport(&subscriptionsServer),
					},
				},
				armMonitorClientOptions: &arm.ClientOptions{
					ClientOptions: azcore.ClientOptions{
						Transport: armmonitorfake.NewServerFactoryTransport(&monitorServerFactory),
					},
				},
				subscriptions: map[string]*azureSubscription{
					"subscriptionId1": {SubscriptionID: "subscriptionId1"},
				},
				resources: map[string]map[string]*azureResource{
					"subscriptionId1": {},
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
			))
		})

	}
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

func getResourcesMockData(tags bool) []armresources.ClientListResponse {
	id1, id2, id3, location1, name1, type1 := "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2", "/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3", "location1", "name1", "type1"

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
	return []armresources.ClientListResponse{
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
	}
}

func getMetricsDefinitionsMockData() map[string][]armmonitor.MetricDefinitionsClientListResponse {
	name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension1, dimension2 := "metric1",
		"metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension1", "dimension2"

	return map[string][]armmonitor.MetricDefinitionsClientListResponse{
		"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1": {
			{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						{
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
	}
}

func TestAzureScraperClientOptions(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	tests := []struct {
		name   string
		fields fields
		want   *arm.ClientOptions
	}{
		{
			name: "AzureCloud_options",
			fields: fields{
				cfg: &Config{
					Cloud: azureCloud,
				},
			},
			want: &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Cloud: cloud.AzurePublic,
				},
			},
		},
		{
			name: "AzureGovernmentCloud_options",
			fields: fields{
				cfg: &Config{
					Cloud: azureGovernmentCloud,
				},
			},
			want: &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Cloud: cloud.AzureGovernment,
				},
			},
		},
		{
			name: "AzureChinaCloud_options",
			fields: fields{
				cfg: &Config{
					Cloud: azureChinaCloud,
				},
			},
			want: &arm.ClientOptions{
				ClientOptions: azcore.ClientOptions{
					Cloud: cloud.AzureChina,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &azureScraper{
				cfg: tt.fields.cfg,
			}
			if got := s.getArmClientOptions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArmClientOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
