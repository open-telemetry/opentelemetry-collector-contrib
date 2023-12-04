// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
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

	scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
	require.Len(t, scraper.resources, 0)
}

func azIDCredentialsFuncMock(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error) {
	return &azidentity.ClientSecretCredential{}, nil
}

func azIDWorkloadFuncMock(*azidentity.WorkloadIdentityCredentialOptions) (*azidentity.WorkloadIdentityCredential, error) {
	return &azidentity.WorkloadIdentityCredential{}, nil
}

func armClientFuncMock(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error) {
	return &armresources.Client{}, nil
}

func armMonitorDefinitionsClientFuncMock(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error) {
	return &armmonitor.MetricDefinitionsClient{}, nil
}

func armMonitorMetricsClientFuncMock(string, azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error) {
	return &armmonitor.MetricsClient{}, nil
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
					cfg:                             cfg,
					azIDCredentialsFunc:             azIDCredentialsFuncMock,
					azIDWorkloadFunc:                azIDWorkloadFuncMock,
					armClientFunc:                   armClientFuncMock,
					armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
					armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
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
					ScraperControllerSettings:     cfg.ScraperControllerSettings,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                servicePrincipal,
				}
				s := &azureScraper{
					cfg:                             customCfg,
					azIDCredentialsFunc:             azIDCredentialsFuncMock,
					azIDWorkloadFunc:                azIDWorkloadFuncMock,
					armClientFunc:                   armClientFuncMock,
					armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
					armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
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
					ScraperControllerSettings:     cfg.ScraperControllerSettings,
					MetricsBuilderConfig:          metadata.DefaultMetricsBuilderConfig(),
					CacheResources:                24 * 60 * 60,
					CacheResourcesDefinitions:     24 * 60 * 60,
					MaximumNumberOfMetricsInACall: 20,
					Services:                      monitorServices,
					Authentication:                workloadIdentity,
				}
				s := &azureScraper{
					cfg:                             customCfg,
					azIDCredentialsFunc:             azIDCredentialsFuncMock,
					azIDWorkloadFunc:                azIDWorkloadFuncMock,
					armClientFunc:                   armClientFuncMock,
					armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
					armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
				}

				if err := s.start(context.Background(), componenttest.NewNopHost()); err != nil {
					t.Errorf("azureScraper.start() error = %v", err)
				}
				require.NotNil(t, s.cred)
				require.IsType(t, &azidentity.WorkloadIdentityCredential{}, s.cred)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

type armClientMock struct {
	current int
	pages   []armresources.ClientListResponse
}

func (acm *armClientMock) NewListPager(_ *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armresources.ClientListResponse]{
		More: func(page armresources.ClientListResponse) bool {
			return acm.current < len(acm.pages)
		},
		Fetcher: func(ctx context.Context, page *armresources.ClientListResponse) (armresources.ClientListResponse, error) {
			currentPage := acm.pages[acm.current]
			acm.current++
			return currentPage, nil
		},
	})
}

type metricsDefinitionsClientMock struct {
	current map[string]int
	pages   map[string][]armmonitor.MetricDefinitionsClientListResponse
}

func (mdcm *metricsDefinitionsClientMock) NewListPager(resourceURI string, _ *armmonitor.MetricDefinitionsClientListOptions) *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armmonitor.MetricDefinitionsClientListResponse]{
		More: func(page armmonitor.MetricDefinitionsClientListResponse) bool {
			return mdcm.current[resourceURI] < len(mdcm.pages[resourceURI])
		},
		Fetcher: func(ctx context.Context, page *armmonitor.MetricDefinitionsClientListResponse) (armmonitor.MetricDefinitionsClientListResponse, error) {
			currentPage := mdcm.pages[resourceURI][mdcm.current[resourceURI]]
			mdcm.current[resourceURI]++
			return currentPage, nil
		},
	})
}

type metricsValuesClientMock struct {
	lists map[string]map[string]armmonitor.MetricsClientListResponse
}

func (mvcm metricsValuesClientMock) List(_ context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
	return mvcm.lists[resourceURI][*options.Metricnames], nil
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
			settings := receivertest.NewNopCreateSettings()

			armClientMock := &armClientMock{
				current: 0,
				pages:   getResourcesMockData(tt.fields.cfg.AppendTagsAsAttributes),
			}

			counters, pages := getMetricsDefinitionsMockData()

			metricsDefinitionsClientMock := &metricsDefinitionsClientMock{
				current: counters,
				pages:   pages,
			}

			metricsValuesClientMock := &metricsValuesClientMock{
				lists: getMetricsValuesMockData(),
			}

			s := &azureScraper{
				cfg:                      tt.fields.cfg,
				clientResources:          armClientMock,
				clientMetricsDefinitions: metricsDefinitionsClientMock,
				clientMetricsValues:      metricsValuesClientMock,
				mb:                       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
				mutex:                    &sync.Mutex{},
			}
			s.resources = map[string]*azureResource{}

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

func getResourcesMockData(tags bool) []armresources.ClientListResponse {
	id1, id2, id3, location1, name1, type1 := "/resourceGroups/group1/resourceId1",
		"/resourceGroups/group1/resourceId2", "/resourceGroups/group1/resourceId3", "location1", "name1", "type1"

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

func getMetricsDefinitionsMockData() (map[string]int, map[string][]armmonitor.MetricDefinitionsClientListResponse) {
	name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension1, dimension2 := "metric1",
		"metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension1", "dimension2"

	counters := map[string]int{
		"/resourceGroups/group1/resourceId1": 0,
		"/resourceGroups/group1/resourceId2": 0,
		"/resourceGroups/group1/resourceId3": 0,
	}

	pages := map[string][]armmonitor.MetricDefinitionsClientListResponse{
		"/resourceGroups/group1/resourceId1": {
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
		"/resourceGroups/group1/resourceId2": {
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
		"/resourceGroups/group1/resourceId3": {
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
	return counters, pages
}

func getMetricsValuesMockData() map[string]map[string]armmonitor.MetricsClientListResponse {
	name1, name2, name3, name4, name5, name6, name7, dimension1, dimension2, dimensionValue := "metric1", "metric2",
		"metric3", "metric4", "metric5", "metric6", "metric7", "dimension1", "dimension2", "dimension value"
	var unit1 armmonitor.Unit = "unit1"
	var value1 float64 = 1

	return map[string]map[string]armmonitor.MetricsClientListResponse{
		"/resourceGroups/group1/resourceId1": {
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
		"/resourceGroups/group1/resourceId2": {
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
		"/resourceGroups/group1/resourceId3": {
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
