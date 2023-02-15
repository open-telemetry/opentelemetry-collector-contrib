// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorreceiver

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
	require.Len(t, scraper.resources, 0)
}

func azIdCredentialsFuncMock(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error) {
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
				azIdCredentialsFunc:             azIdCredentialsFuncMock,
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

func (acm *armClientMock) NewListPager(options *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse] {
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

func (mdcm *metricsDefinitionsClientMock) NewListPager(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse] {
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
	lists map[string]armmonitor.MetricsClientListResponse
}

func (mvcm metricsValuesClientMock) List(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
	return mvcm.lists[resourceURI], nil
}

func TestAzureScraperScrape(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx context.Context
	}
	cfg := createDefaultConfig().(*Config)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				cfg: cfg,
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
				pages:   getResourcesMockData(),
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
			}
			s.resources = map[string]*azureResource{}

			metrics, err := s.scrape(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValues(t, 11, metrics.MetricCount(), "Scraper should have return expected number of metrics")
		})
	}
}

func getResourcesMockData() []armresources.ClientListResponse {
	id1, id2, id3 := "resourceId1", "resourceId2", "resourceId3"

	return []armresources.ClientListResponse{
		armresources.ClientListResponse{
			ResourceListResult: armresources.ResourceListResult{
				Value: []*armresources.GenericResourceExpanded{
					&armresources.GenericResourceExpanded{
						ID: &id1,
					},
					&armresources.GenericResourceExpanded{
						ID: &id2,
					},
				},
			},
		},
		armresources.ClientListResponse{
			ResourceListResult: armresources.ResourceListResult{
				Value: []*armresources.GenericResourceExpanded{
					&armresources.GenericResourceExpanded{
						ID: &id3,
					},
				},
			},
		},
	}
}

func getMetricsDefinitionsMockData() (map[string]int, map[string][]armmonitor.MetricDefinitionsClientListResponse) {
	name1 := "metric1"
	timeGrain := "PT1M"

	counters := map[string]int{
		"resourceId1": 0,
		"resourceId2": 0,
		"resourceId3": 0,
	}

	pages := map[string][]armmonitor.MetricDefinitionsClientListResponse{
		"resourceId1": []armmonitor.MetricDefinitionsClientListResponse{
			armmonitor.MetricDefinitionsClientListResponse{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						&armmonitor.MetricDefinition{
							Name: &armmonitor.LocalizableString{
								Value: &name1,
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
		"resourceId2": []armmonitor.MetricDefinitionsClientListResponse{
			armmonitor.MetricDefinitionsClientListResponse{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						&armmonitor.MetricDefinition{
							Name: &armmonitor.LocalizableString{
								Value: &name1,
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
		"resourceId3": []armmonitor.MetricDefinitionsClientListResponse{
			armmonitor.MetricDefinitionsClientListResponse{
				MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
					Value: []*armmonitor.MetricDefinition{
						&armmonitor.MetricDefinition{
							Name: &armmonitor.LocalizableString{
								Value: &name1,
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
	return counters, pages
}

func getMetricsValuesMockData() map[string]armmonitor.MetricsClientListResponse {
	name1 := "metric1"
	var unit1 armmonitor.MetricUnit = "unit1"
	var value1 float64 = 1

	return map[string]armmonitor.MetricsClientListResponse{
		"resourceId1": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
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
		"resourceId2": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
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
		"resourceId3": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
										Count: &value1,
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
