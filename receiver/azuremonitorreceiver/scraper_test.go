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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
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

func armClientFuncMock(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error) {
	return &armresources.Client{}, nil
}

func armMonitorDefinitionsClientFuncMock(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error) {
	return &armmonitor.MetricDefinitionsClient{}, nil
}

func armMonitorMetricsClientFuncMock(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error) {
	return &armmonitor.MetricsClient{}, nil
}

func TestAzureScraperStart(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx  context.Context
		host component.Host
	}

	cfg := createDefaultConfig().(*Config)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "1st",
			fields: fields{
				cfg: cfg,
			},
			args: args{
				ctx:  context.Background(),
				host: componenttest.NewNopHost(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &azureScraper{
				cfg:                             tt.fields.cfg,
				azIDCredentialsFunc:             azIDCredentialsFuncMock,
				armClientFunc:                   armClientFuncMock,
				armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
				armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
			}

			if err := s.start(tt.args.ctx, tt.args.host); (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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

func (mvcm metricsValuesClientMock) List(ctx context.Context, resourceURI string, _ *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
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
	id1, id2, id3, location1 := "resourceId1", "resourceId2", "resourceId3", "location1"

	resourceID1 := armresources.GenericResourceExpanded{
		ID:       &id1,
		Location: &location1,
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
						ID: &id2,
					},
				},
			},
		},
		{
			ResourceListResult: armresources.ResourceListResult{
				Value: []*armresources.GenericResourceExpanded{
					{
						ID: &id3,
					},
				},
			},
		},
	}
}

func getMetricsDefinitionsMockData() (map[string]int, map[string][]armmonitor.MetricDefinitionsClientListResponse) {
	name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension := "metric1",
		"metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension"

	counters := map[string]int{
		"resourceId1": 0,
		"resourceId2": 0,
		"resourceId3": 0,
	}

	pages := map[string][]armmonitor.MetricDefinitionsClientListResponse{
		"resourceId1": {
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
		"resourceId2": {
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
									Value: &dimension,
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
									Value: &dimension,
								},
							},
						},
					},
				},
			},
		},
		"resourceId3": {
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
									Value: &dimension,
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
	name1, name2, name3, name4, name5, name6, name7, dimension, dimensionValue := "metric1", "metric2",
		"metric3", "metric4", "metric5", "metric6", "metric7", "dimension", "dimension value"
	var unit1 armmonitor.MetricUnit = "unit1"
	var value1 float64 = 1

	return map[string]map[string]armmonitor.MetricsClientListResponse{
		"resourceId1": {
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
		"resourceId2": {
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
			strings.Join([]string{name5, name6}, ","): {
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
												Value: &dimension,
											},
											Value: &dimensionValue,
										},
									},
								},
							},
						},
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
												Value: &dimension,
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
		"resourceId3": {
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
												Value: &dimension,
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
