// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	armmonitorfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armresourcesfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
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
	timeMock := getTimeMock()

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
					time:                            timeMock,
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
					time:                            timeMock,
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
					time:                            timeMock,
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
					time:                            timeMock,
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
					time:                            timeMock,
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
			settings := receivertest.NewNopSettings(metadata.Type)

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
				time:                     getTimeMock(),
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

func TestAzureScraperScrapeFilterMetrics(t *testing.T) {
	fakeSubID := "/subscription/azuremonitor-receiver"
	fakeCreds := &azfake.TokenCredential{}
	metricNamespace1, metricNamespace2 := "Microsoft.ServiceA/namespace1", "Microsoft.ServiceB/namespace2"
	metricName1, metricName2, metricName3 := "ConnectionsTotal", "IncommingMessages", "TransferedBytes"
	metricAggregation1, metricAggregation2, metricAggregation3 := "Count", "Maximum", "Minimum"
	cfgLimitedMertics := createDefaultConfig().(*Config)
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
		fakeResourceServer := &armresourcesfake.Server{
			NewListPager: func(*armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
				name := "resource-name"
				id1 := fakeSubID + "/resourceGroups/resource-group/providers/" + metricNamespace1 + "/" + name
				id2 := fakeSubID + "/resourceGroups/resource-group/providers/" + metricNamespace2 + "/" + name
				location := "location-name"
				resp.AddPage(http.StatusOK, armresources.ClientListResponse{
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
				}, nil)
				resp.AddPage(http.StatusOK, armresources.ClientListResponse{
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
				}, nil)
				return
			},
		}
		armClientMock, err := armresources.NewClient(fakeSubID, fakeCreds, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: armresourcesfake.NewServerTransport(fakeResourceServer),
			},
		})
		require.NoError(t, err, "should create fake client")

		fakeMetricDefinitionsServer := &armmonitorfake.MetricDefinitionsServer{
			NewListPager: func(uri string, _ *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
				timeGrain := "PT1M"
				if strings.Contains(uri, metricNamespace1) {
					resp.AddPage(http.StatusOK, armmonitor.MetricDefinitionsClientListResponse{
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
					}, nil)
				}
				if strings.Contains(uri, metricNamespace2) {
					resp.AddPage(http.StatusOK, armmonitor.MetricDefinitionsClientListResponse{
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
					}, nil)
				}
				return
			},
		}
		metricsDefinitionsClientMock, err := armmonitor.NewMetricDefinitionsClient(fakeSubID, fakeCreds, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: armmonitorfake.NewMetricDefinitionsServerTransport(fakeMetricDefinitionsServer),
			},
		})
		require.NoError(t, err, "should create fake metric definition client")

		fakeMetricsServer := &armmonitorfake.MetricsServer{
			List: func(_ context.Context, _ string, opts *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
				var unit armmonitor.Unit = "u"
				var valueCount float64 = 11
				valueMaximum := 123.45
				valueMinimum := 0.1
				switch *opts.Metricnames {
				case metricName1:
					resp.SetResponse(http.StatusOK, armmonitor.MetricsClientListResponse{
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
					}, nil)
				case metricName2:
					resp.SetResponse(http.StatusOK, armmonitor.MetricsClientListResponse{
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
					}, nil)
				case metricName3:
					resp.SetResponse(http.StatusOK, armmonitor.MetricsClientListResponse{
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
					}, nil)
				}
				return
			},
		}
		metricsClientMock, err := armmonitor.NewMetricsClient(fakeSubID, fakeCreds, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: armmonitorfake.NewMetricsServerTransport(fakeMetricsServer),
			},
		})
		require.NoError(t, err, "should create fake metric client")

		settings := receivertest.NewNopSettings(metadata.Type)
		s := &azureScraper{
			cfg:                      cfgLimitedMertics,
			clientResources:          armClientMock,
			clientMetricsDefinitions: metricsDefinitionsClientMock,
			clientMetricsValues:      metricsClientMock,
			mb:                       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
			mutex:                    &sync.Mutex{},
			time:                     getTimeMock(),
			resources:                map[string]*azureResource{},
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
		))
	})
}

func TestAzureScraperScrapeHonorTimeGrain(t *testing.T) {
	getTestScraper := func() *azureScraper {
		armClientMock := &armClientMock{
			current: 0,
			pages:   getResourcesMockData(false),
		}
		counters, pages := getMetricsDefinitionsMockData()
		metricsDefinitionsClientMock := &metricsDefinitionsClientMock{
			current: counters,
			pages:   pages,
		}
		metricsValuesClientMock := &metricsValuesClientMock{
			lists: getMetricsValuesMockData(),
		}

		return &azureScraper{
			cfg:                      createDefaultConfig().(*Config),
			clientResources:          armClientMock,
			clientMetricsDefinitions: metricsDefinitionsClientMock,
			clientMetricsValues:      metricsValuesClientMock,
			mb: metadata.NewMetricsBuilder(
				metadata.DefaultMetricsBuilderConfig(),
				receivertest.NewNopSettings(receivertest.NopType),
			),
			mutex:     &sync.Mutex{},
			resources: map[string]*azureResource{},
			time:      getTimeMock(),
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
	namespace1, namespace2, name1, name2, name3, name4, name5, name6, name7, timeGrain1, timeGrain2, dimension1, dimension2 := "namespace1",
		"namespace2", "metric1", "metric2", "metric3", "metric4", "metric5", "metric6", "metric7", "PT1M", "PT1H", "dimension1", "dimension2"

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
		"/resourceGroups/group1/resourceId2": {
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
		"/resourceGroups/group1/resourceId3": {
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
