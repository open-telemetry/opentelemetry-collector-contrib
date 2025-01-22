// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
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
					ControllerConfig:              cfg.ControllerConfig,
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
					ControllerConfig:              cfg.ControllerConfig,
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
					cfg:                             customCfg,
					azIDCredentialsFunc:             azIDCredentialsFuncMock,
					azManagedIdentityFunc:           azManagedIdentityFuncMock,
					armClientFunc:                   armClientFuncMock,
					armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
					armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
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
					cfg:                             customCfg,
					azIDCredentialsFunc:             azIDCredentialsFuncMock,
					azDefaultCredentialsFunc:        azDefaultCredentialsFuncMock,
					armClientFunc:                   armClientFuncMock,
					armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
					armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
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

type armClientMock struct {
	current int
	pages   []armresources.ClientListResponse
}

func (acm *armClientMock) NewListPager(_ *armresources.ClientListOptions) *runtime.Pager[armresources.ClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armresources.ClientListResponse]{
		More: func(armresources.ClientListResponse) bool {
			return acm.current < len(acm.pages)
		},
		Fetcher: func(context.Context, *armresources.ClientListResponse) (armresources.ClientListResponse, error) {
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
		More: func(armmonitor.MetricDefinitionsClientListResponse) bool {
			return mdcm.current[resourceURI] < len(mdcm.pages[resourceURI])
		},
		Fetcher: func(context.Context, *armmonitor.MetricDefinitionsClientListResponse) (armmonitor.MetricDefinitionsClientListResponse, error) {
			currentPage := mdcm.pages[resourceURI][mdcm.current[resourceURI]]
			mdcm.current[resourceURI]++
			return currentPage, nil
		},
	})
}

type metricsValuesClientMock struct{}

func (mvcm metricsValuesClientMock) List(_ context.Context, _ string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
	var unit1 armmonitor.Unit = "unit1"

	amMetrics := []*armmonitor.Metric{}
	for _, name := range strings.Split(*options.Metricnames, ",") {
		amMetric := &armmonitor.Metric{
			Name: &armmonitor.LocalizableString{
				Value: &name,
			},
			Unit: &unit1,
			Timeseries: []*armmonitor.TimeSeriesElement{
				{
					Data: []*armmonitor.MetricValue{
						mvcm.getAMDataPoints(*options.Aggregation),
					},
				},
			},
		}
		amMetrics = append(amMetrics, amMetric)

		switch name {
		case "metric5":
			amMetric.Timeseries[0].Metadatavalues = mvcm.getAMMetadataValues(2)

		case "metric6":
			amMetric.Timeseries[0].Metadatavalues = mvcm.getAMMetadataValues(1)

		case "metric7":
			amMetric.Timeseries[0].Data[0] = mvcm.getAMDataPoints("Count")
			amMetric.Timeseries[0].Metadatavalues = mvcm.getAMMetadataValues(1)
		}
	}

	return armmonitor.MetricsClientListResponse{
		Response: armmonitor.Response{Value: amMetrics},
	}, nil
}

func (mvcm metricsValuesClientMock) getAMDataPoints(aggregations string) *armmonitor.MetricValue {
	var value1 float64 = 1

	amPoints := &armmonitor.MetricValue{}
	for _, aggregation := range strings.Split(aggregations, ",") {
		switch aggregation {
		case "Average":
			amPoints.Average = &value1
		case "Count":
			amPoints.Count = &value1
		case "Maximum":
			amPoints.Maximum = &value1
		case "Minimum":
			amPoints.Minimum = &value1
		case "Total":
			amPoints.Total = &value1
		}
	}

	return amPoints
}

func (mvcm metricsValuesClientMock) getAMMetadataValues(n int) []*armmonitor.MetadataValue {
	dimensionValue := "dimension value"

	out := make([]*armmonitor.MetadataValue, n)
	for idx := range out {
		dimension := fmt.Sprintf("dimension%d", idx+1)
		out[idx] = &armmonitor.MetadataValue{
			Name: &armmonitor.LocalizableString{
				Value: &dimension,
			},
			Value: &dimensionValue,
		}
	}

	return out
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

	cfgLimitedMertics := createDefaultConfig().(*Config)
	cfgLimitedMertics.MaximumNumberOfMetricsInACall = 2
	cfgLimitedMertics.Metrics = []string{"metric1", "metric3/total", "metric4/average", "metric4/minimum", "metric4/maximum"}

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
		{
			name: "metrics_filtered",
			fields: fields{
				cfg: cfgLimitedMertics,
			},
			args: args{
				ctx: context.Background(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopSettings()

			armClientMock := &armClientMock{
				current: 0,
				pages:   getResourcesMockData(tt.fields.cfg.AppendTagsAsAttributes),
			}

			counters, pages := getMetricsDefinitionsMockData()

			metricsDefinitionsClientMock := &metricsDefinitionsClientMock{
				current: counters,
				pages:   pages,
			}

			metricsValuesClientMock := &metricsValuesClientMock{}

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

func TestIsMetricMatchFilters(t *testing.T) {
	testMetricName := "MetricName1"
	tests := []struct {
		name    string
		filters []string
		want    bool
	}{
		{
			"filters_empty",
			[]string{},
			true,
		},
		{
			"filters_include_metric",
			[]string{"foo", testMetricName, "bar"},
			true,
		},
		{
			"filters_include_metric_ignore_case",
			[]string{"foo", strings.ToLower(testMetricName), "bar"},
			true,
		},
		{
			"filters_include_metric_aggregation",
			[]string{"foo/count", testMetricName + "/total", "bar/total"},
			true,
		},
		{
			"filters_exclude_metric",
			[]string{"foo", "bar"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMetricMatchFilters(testMetricName, tt.filters)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetMetricAggregations(t *testing.T) {
	testMetricName := "MetricName"
	tests := []struct {
		name    string
		filters []string
		want    string
	}{
		{
			"filters_empty",
			[]string{},
			strings.Join(aggregations, ","),
		},
		{
			"filters_include_metric",
			[]string{"foo", testMetricName, "bar"},
			strings.Join(aggregations, ","),
		},
		{
			"filters_include_metric_ignore_case",
			[]string{"foo", strings.ToLower(testMetricName), "bar"},
			strings.Join(aggregations, ","),
		},
		{
			"filters_include_metric_aggregation",
			[]string{"foo/count", testMetricName + "/" + aggregations[0], "bar/total"},
			aggregations[0],
		},
		{
			"filters_include_metric_aggregation_ignore_case",
			[]string{"foo/count", testMetricName + "/" + strings.ToLower(aggregations[0]), "bar/total"},
			aggregations[0],
		},
		{
			"filters_include_metric_multiple_aggregations",
			[]string{"foo/count", testMetricName + "/" + aggregations[0], testMetricName + "/" + aggregations[2]},
			aggregations[0] + "," + aggregations[2],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMetricAggregations(testMetricName, tt.filters)
			require.Equal(t, tt.want, got)
		})
	}
}
