// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"
import (
	"context"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

type queryResourcesResponseMockParams struct {
	subscriptionID  string
	metricNamespace string
	metricNames     []string
	resourceIDs     azmetrics.ResourceIDList
}

func (p queryResourcesResponseMockParams) Evaluate(subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList) bool {
	metricNamesParamClone := slices.Clone(metricNames)
	metricNamesClone := slices.Clone(p.metricNames)
	slices.Sort(metricNamesParamClone)
	slices.Sort(metricNamesClone)

	resourceIDsParamClone := slices.Clone(resourceIDs.ResourceIDs)
	resourceIDsClone := slices.Clone(p.resourceIDs.ResourceIDs)
	slices.Sort(resourceIDsParamClone)
	slices.Sort(resourceIDsClone)

	return p.subscriptionID == subscriptionID && p.metricNamespace == metricNamespace &&
		reflect.DeepEqual(metricNamesClone, metricNamesParamClone) && reflect.DeepEqual(resourceIDsClone, resourceIDsParamClone)
}

type queryResourcesResponseMock struct {
	params   queryResourcesResponseMockParams
	response azmetrics.QueryResourcesResponse
}

func newMockMetricsQueryResponse(metricsByParam []queryResourcesResponseMock) func(ctx context.Context, subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList, options *azmetrics.QueryResourcesOptions) (resp azfake.Responder[azmetrics.QueryResourcesResponse], errResp azfake.ErrorResponder) {
	return func(_ context.Context, subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList, _ *azmetrics.QueryResourcesOptions) (resp azfake.Responder[azmetrics.QueryResourcesResponse], errResp azfake.ErrorResponder) {
		for _, param := range metricsByParam {
			if param.params.Evaluate(subscriptionID, metricNamespace, metricNames, resourceIDs) {
				resp.SetResponse(http.StatusOK, param.response, nil)
				return
			}
		}
		errResp.SetResponseError(http.StatusNotImplemented, "error from tests")
		return
	}
}

func getMetricsQueryResponseMockData() []queryResourcesResponseMock {
	return []queryResourcesResponseMock{
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId3",
				metricNamespace: "type1",
				metricNames:     []string{"metric7"},
				resourceIDs:     azmetrics.ResourceIDList{ResourceIDs: []string{"/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1"}},
			},
			response: azmetrics.QueryResourcesResponse{
				MetricResults: azmetrics.MetricResults{Values: []azmetrics.MetricData{
					{
						ResourceID: to.Ptr("/subscriptions/subscriptionId3/resourceGroups/group1/resourceId1"),
						Values: []azmetrics.Metric{
							{
								Name: &azmetrics.LocalizableString{
									Value: to.Ptr("metric7"),
								},
								Unit: to.Ptr(azmetrics.MetricUnitBitsPerSecond),
								TimeSeries: []azmetrics.TimeSeriesElement{
									{
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
									},
								},
							},
						},
					},
				}},
			},
		},
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId1",
				metricNamespace: "type1",
				metricNames:     []string{"metric1", "metric2"},
				resourceIDs: azmetrics.ResourceIDList{ResourceIDs: []string{
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
				}},
			},
			response: azmetrics.QueryResourcesResponse{
				MetricResults: azmetrics.MetricResults{Values: []azmetrics.MetricData{
					{
						ResourceID: to.Ptr("/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1"),
						Values: []azmetrics.Metric{
							{
								Name: &azmetrics.LocalizableString{
									Value: to.Ptr("metric1"),
								},
								Unit: to.Ptr(azmetrics.MetricUnitPercent),
								TimeSeries: []azmetrics.TimeSeriesElement{
									{
										Data: []azmetrics.MetricValue{
											{
												TimeStamp: to.Ptr(time.Now()),
												Average:   to.Ptr(1.),
												Count:     to.Ptr(1.),
												Maximum:   to.Ptr(1.),
												Minimum:   to.Ptr(1.),
												Total:     to.Ptr(1.),
											},
										},
									},
								},
							},
							{
								Name: &azmetrics.LocalizableString{
									Value: to.Ptr("metric2"),
								},
								Unit: to.Ptr(azmetrics.MetricUnitCount),
								TimeSeries: []azmetrics.TimeSeriesElement{
									{
										Data: []azmetrics.MetricValue{
											{
												TimeStamp: to.Ptr(time.Now()),
												Average:   to.Ptr(1.),
												Count:     to.Ptr(1.),
												Maximum:   to.Ptr(1.),
												Minimum:   to.Ptr(1.),
												Total:     to.Ptr(1.),
											},
										},
									},
								},
							},
						},
					},
				}},
			},
		},
		{
			params: queryResourcesResponseMockParams{
				subscriptionID:  "subscriptionId1",
				metricNamespace: "type1",
				metricNames:     []string{"metric3"},
				resourceIDs: azmetrics.ResourceIDList{ResourceIDs: []string{
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId1",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2",
					"/subscriptions/subscriptionId1/resourceGroups/group1/resourceId3",
				}},
			},
			response: azmetrics.QueryResourcesResponse{
				MetricResults: azmetrics.MetricResults{Values: []azmetrics.MetricData{
					{
						ResourceID: to.Ptr("/subscriptions/subscriptionId1/resourceGroups/group1/resourceId2"),
						Values: []azmetrics.Metric{
							{
								Name: &azmetrics.LocalizableString{
									Value: to.Ptr("metric3"),
								},
								Unit: to.Ptr(azmetrics.MetricUnitBytes),
								TimeSeries: []azmetrics.TimeSeriesElement{
									{
										Data: []azmetrics.MetricValue{
											{
												TimeStamp: to.Ptr(time.Now()),
												Average:   to.Ptr(1.),
												Count:     to.Ptr(1.),
												Maximum:   to.Ptr(1.),
												Minimum:   to.Ptr(1.),
												Total:     to.Ptr(1.),
											},
										},
									},
								},
							},
						},
					},
				}},
			},
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
				getSubscriptionByIDMockData(),
				getSubscriptionsMockData(),
				getResourcesMockData(tt.fields.cfg.AppendTagsAsAttributes),
				getMetricsDefinitionsMockData(),
				nil,
				getMetricsQueryResponseMockData(),
			)

			s := &azureBatchScraper{
				cfg:                   tt.fields.cfg,
				mb:                    metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
				mutex:                 &sync.Mutex{},
				time:                  getTimeMock(),
				clientOptionsResolver: optionsResolver,
				settings:              settings.TelemetrySettings,

				// From there, initialize everything that is normally initialized in start() func
				subscriptions: map[string]*azureSubscription{
					"subscriptionId1": {SubscriptionID: "subscriptionId1"},
					"subscriptionId3": {SubscriptionID: "subscriptionId3"},
				},
				resources: map[string]map[string]*azureResource{
					"subscriptionId1": {},
					"subscriptionId3": {},
				},
				regions: map[string]map[string]struct{}{
					"subscriptionId1": {"location1": {}},
					"subscriptionId3": {"location1": {}},
				},
				resourceTypes: map[string]map[string]*azureType{
					"subscriptionId1": {},
					"subscriptionId3": {},
				},
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
