// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	azmetricsfake "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	armmonitorfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	armresourcesfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	armsubscriptionsfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions/fake"
)

// newMockSubscriptionsListPager is a helper function to create a list pager for subscriptions.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockSubscriptionsListPager(subscriptionsPages []armsubscriptions.ClientListResponse) func(options *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
	return func(_ *armsubscriptions.ClientListOptions) (resp azfake.PagerResponder[armsubscriptions.ClientListResponse]) {
		for _, page := range subscriptionsPages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return resp
	}
}

// newMockSubscriptionGet is a helper function to create a get subscription API.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockSubscriptionGet(subscriptionsByID map[string]armsubscriptions.ClientGetResponse) func(ctx context.Context, subscriptionID string, options *armsubscriptions.ClientGetOptions) (resp azfake.Responder[armsubscriptions.ClientGetResponse], errResp azfake.ErrorResponder) {
	return func(_ context.Context, subscriptionID string, _ *armsubscriptions.ClientGetOptions) (resp azfake.Responder[armsubscriptions.ClientGetResponse], errResp azfake.ErrorResponder) {
		resp.SetResponse(http.StatusOK, subscriptionsByID[subscriptionID], nil)
		return resp, errResp
	}
}

// newMockResourcesListPager is a helper function to create a list pager for resources.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockResourcesListPager(resourcesPages []armresources.ClientListResponse) func(options *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
	return func(_ *armresources.ClientListOptions) (resp azfake.PagerResponder[armresources.ClientListResponse]) {
		for _, page := range resourcesPages {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return resp
	}
}

// newMockMetricsDefinitionListPager is a helper function to create a list pager for metrics definitions.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockMetricsDefinitionListPager(metricDefinitionsPagesByResourceURI map[string][]armmonitor.MetricDefinitionsClientListResponse) func(resourceURI string, options *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
	return func(resourceURI string, _ *armmonitor.MetricDefinitionsClientListOptions) (resp azfake.PagerResponder[armmonitor.MetricDefinitionsClientListResponse]) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		for _, page := range metricDefinitionsPagesByResourceURI[resourceURI] {
			resp.AddPage(http.StatusOK, page, nil)
		}
		return resp
	}
}

// newMockMetricList is a helper function to create a list metrics API.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockMetricList(metricsByResourceURIAndMetricName map[string]map[string]armmonitor.MetricsClientListResponse) func(ctx context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
	return func(_ context.Context, resourceURI string, options *armmonitor.MetricsClientListOptions) (resp azfake.Responder[armmonitor.MetricsClientListResponse], errResp azfake.ErrorResponder) {
		resourceURI = fmt.Sprintf("/%s", resourceURI) // Hack the fake API as it's not taking starting slash from called request
		resp.SetResponse(http.StatusOK, metricsByResourceURIAndMetricName[resourceURI][*options.Metricnames], nil)
		return resp, errResp
	}
}

// newMockMetricsQueryResponse is a helper function to create a map of query parameters to response.
// Don't use it, it's designed to be called in the newMockClientOptionsResolver ctor only.
func newMockMetricsQueryResponse(metricsByParam []queryResourcesResponseMock) func(ctx context.Context, subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList, options *azmetrics.QueryResourcesOptions) (resp azfake.Responder[azmetrics.QueryResourcesResponse], errResp azfake.ErrorResponder) {
	return func(_ context.Context, subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList, _ *azmetrics.QueryResourcesOptions) (resp azfake.Responder[azmetrics.QueryResourcesResponse], errResp azfake.ErrorResponder) {
		for _, param := range metricsByParam {
			if param.params.Evaluate(subscriptionID, metricNamespace, metricNames, resourceIDs.ResourceIDs) {
				resp.SetResponse(http.StatusOK, param.response, nil)
				return resp, errResp
			}
		}
		errResp.SetResponseError(http.StatusNotImplemented, "error from tests")
		return resp, errResp
	}
}

// queryResourcesResponseMockParams is used to mock the response of the metrics query API.
// See the newMockMetricsQueryResponse ctor for more details.
type queryResourcesResponseMockParams struct {
	subscriptionID  string
	metricNamespace string
	metricNames     []string
	resourceIDs     []string
}

// Evaluate returns true if the params match the given params.
// See the queryResourcesResponseMock struct for more details.
func (p queryResourcesResponseMockParams) Evaluate(subscriptionID, metricNamespace string, metricNames, resourceIDs []string) bool {
	metricNamesParamClone := slices.Clone(metricNames)
	metricNamesClone := slices.Clone(p.metricNames)
	slices.Sort(metricNamesParamClone)
	slices.Sort(metricNamesClone)

	resourceIDsParamClone := slices.Clone(resourceIDs)
	resourceIDsClone := slices.Clone(p.resourceIDs)
	slices.Sort(resourceIDsParamClone)
	slices.Sort(resourceIDsClone)

	return p.subscriptionID == subscriptionID && p.metricNamespace == metricNamespace &&
		reflect.DeepEqual(metricNamesClone, metricNamesParamClone) && reflect.DeepEqual(resourceIDsClone, resourceIDsParamClone)
}

// queryResourcesResponseMock is used to mock the response of the metrics query API.
// It is composed of a list of query parameters and a response.
// Params will be evaluated against actual params given, and if it matches, the response will be returned.
type queryResourcesResponseMock struct {
	params   queryResourcesResponseMockParams
	response azmetrics.QueryResourcesResponse
}

// mockClientOptionsResolver is a mock implementation of ClientOptionsResolver.
// It is used to mock the Azure API calls.
type mockClientOptionsResolver struct {
	armResourcesClientOptions     map[string]*arm.ClientOptions
	armSubscriptionsClientOptions *arm.ClientOptions
	armMonitorClientOptions       *arm.ClientOptions
	azMetricsClientOptions        *azmetrics.ClientOptions
}

// newMockClientOptionsResolver is an option resolver that will generate mocking client options for each Azure API.
// Indeed, the way to mock Azure API is to provide a fake server that will return the expected data.
// The fake server is built with "fake" package from Azure SDK for Go, and is set in the client options, via the transport.
// This ctor takes the mock data in that order:
// - subscriptions get responses stored by ID
// - subscriptions list response
// - resources stored by subscription ID
// - metrics definitions stored by resource URI
// - metrics values stored by resource URI and metric name
func newMockClientOptionsResolver(
	subscriptionsGetResponsesByID map[string]armsubscriptions.ClientGetResponse,
	subscriptionsListResponse []armsubscriptions.ClientListResponse,
	resources map[string][]armresources.ClientListResponse,
	metricsDefinitions map[string][]armmonitor.MetricDefinitionsClientListResponse,
	metrics map[string]map[string]armmonitor.MetricsClientListResponse,
	metricsQueryResponses []queryResourcesResponseMock,
) ClientOptionsResolver {
	// Init resources client options from resources mock data
	armResourcesClientOptions := make(map[string]*arm.ClientOptions)
	for subID, pages := range resources {
		resourceServer := armresourcesfake.Server{
			NewListPager: newMockResourcesListPager(pages),
		}
		armResourcesClientOptions[subID] = &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Cloud:     cloud.AzurePublic, // Ensure Cloud client options is set. This is important to prevent race condition in the client constructor.
				Transport: armresourcesfake.NewServerTransport(&resourceServer),
			},
		}
	}

	// Init subscriptions client options from subscriptions mock data
	subscriptionsServer := armsubscriptionsfake.Server{
		NewListPager: newMockSubscriptionsListPager(subscriptionsListResponse),
		Get:          newMockSubscriptionGet(subscriptionsGetResponsesByID),
	}
	armSubscriptionsClientOptions := &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud:     cloud.AzurePublic, // Ensure Cloud client options is set. This is important to prevent race condition in the client constructor.
			Transport: armsubscriptionsfake.NewServerTransport(&subscriptionsServer),
		},
	}

	// Init arm monitor client options from subscriptions mock data
	armMonitorServerFactory := armmonitorfake.ServerFactory{
		MetricDefinitionsServer: armmonitorfake.MetricDefinitionsServer{
			NewListPager: newMockMetricsDefinitionListPager(metricsDefinitions),
		},
		MetricsServer: armmonitorfake.MetricsServer{
			List: newMockMetricList(metrics),
		},
	}
	armMonitorClientOptions := &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud:     cloud.AzurePublic, // Ensure Cloud client options is set. This is important to prevent race condition in the client constructor.
			Transport: armmonitorfake.NewServerFactoryTransport(&armMonitorServerFactory),
		},
	}

	// Init az metrics client options from metrics query responses mock data
	azMetricsServer := azmetricsfake.Server{
		QueryResources: newMockMetricsQueryResponse(metricsQueryResponses),
	}
	azMetricsClientOptions := &azmetrics.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud:     cloud.AzurePublic, // Ensure Cloud client options is set. This is important to prevent race condition in the client constructor.
			Transport: azmetricsfake.NewServerTransport(&azMetricsServer),
		},
	}

	return &mockClientOptionsResolver{
		armResourcesClientOptions:     armResourcesClientOptions,
		armSubscriptionsClientOptions: armSubscriptionsClientOptions,
		armMonitorClientOptions:       armMonitorClientOptions,
		azMetricsClientOptions:        azMetricsClientOptions,
	}
}

func (m mockClientOptionsResolver) GetArmResourceClientOptions(subscriptionID string) *arm.ClientOptions {
	return m.armResourcesClientOptions[subscriptionID]
}

func (m mockClientOptionsResolver) GetArmSubscriptionsClientOptions() *arm.ClientOptions {
	return m.armSubscriptionsClientOptions
}

func (m mockClientOptionsResolver) GetArmMonitorClientOptions() *arm.ClientOptions {
	return m.armMonitorClientOptions
}

func (m mockClientOptionsResolver) GetAzMetricsClientOptions() *azmetrics.ClientOptions {
	return m.azMetricsClientOptions
}

// timeMock is used to mock the time.Now() function.
type timeMock struct {
	time time.Time
}

// Now returns the time set in the mock.
func (t *timeMock) Now() time.Time {
	return t.time
}

// =====================================================================================================================
// ============================ Build Azure mock data with simplified input ============================================
// =====================================================================================================================

// newSubscriptionsListMockData is a helper function to create a subscription list response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose armsubscriptions.ClientListResponse.
func newSubscriptionsListMockData(input [][]string) []armsubscriptions.ClientListResponse {
	result := make([]armsubscriptions.ClientListResponse, len(input))
	for i, inputSlice := range input {
		result[i] = armsubscriptions.ClientListResponse{
			SubscriptionListResult: armsubscriptions.SubscriptionListResult{
				Value: make([]*armsubscriptions.Subscription, len(inputSlice)),
			},
		}
		for j, subscriptionID := range inputSlice {
			result[i].Value[j] = &armsubscriptions.Subscription{
				SubscriptionID: &subscriptionID,
			}
		}
	}
	return result
}

// newSubscriptionsByIDMockData is a helper function to create a subscription by id response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose armsubscriptions.ClientGetResponse.
func newSubscriptionsByIDMockData(input map[string]string) map[string]armsubscriptions.ClientGetResponse {
	result := make(map[string]armsubscriptions.ClientGetResponse)
	for subscriptionID, displayName := range input {
		result[subscriptionID] = armsubscriptions.ClientGetResponse{
			Subscription: armsubscriptions.Subscription{
				SubscriptionID: &subscriptionID, DisplayName: &displayName,
			},
		}
	}
	return result
}

// newResourcesMockData is a helper function to create a resource list response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose armsubscriptions.ClientGetResponse.
func newResourcesMockData(inputMap map[string][][]*armresources.GenericResourceExpanded) map[string][]armresources.ClientListResponse {
	result := make(map[string][]armresources.ClientListResponse)
	for subscriptionID, inputSlice1 := range inputMap {
		result[subscriptionID] = make([]armresources.ClientListResponse, len(inputSlice1))
		for i, inputSlice2 := range inputSlice1 {
			result[subscriptionID][i] = armresources.ClientListResponse{
				ResourceListResult: armresources.ResourceListResult{
					Value: inputSlice2,
				},
			}
		}
	}
	return result
}

// metricsDefinitionMockInput is used to mock the response of the metrics definition API.
// Everything is required except dimensions.
type metricsDefinitionMockInput struct {
	namespace  string
	name       string
	timeGrain  string
	dimensions []string
}

// newMetricsDefinitionMockData is a helper function to create metrics definition list response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose armmonitor.MetricDefinitionsClientListResponse.
func newMetricsDefinitionMockData(inputMap map[string][]metricsDefinitionMockInput) map[string][]armmonitor.MetricDefinitionsClientListResponse {
	result := make(map[string][]armmonitor.MetricDefinitionsClientListResponse)
	for resourceID, inputSlice := range inputMap {
		values := make([]*armmonitor.MetricDefinition, len(inputSlice))

		for i, input := range inputSlice {
			toAppend := armmonitor.MetricDefinition{
				Namespace:            &input.namespace,
				Name:                 &armmonitor.LocalizableString{Value: &input.name},
				MetricAvailabilities: []*armmonitor.MetricAvailability{{TimeGrain: &input.timeGrain}},
			}

			for _, dimension := range input.dimensions {
				toAppend.Dimensions = append(toAppend.Dimensions, &armmonitor.LocalizableString{Value: &dimension})
			}
			values[i] = &toAppend
		}

		result[resourceID] = []armmonitor.MetricDefinitionsClientListResponse{{
			MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
				Value: values,
			},
		}}
	}
	return result
}

type queryResourceMockInput struct {
	ResourceID string
	Metrics    []metricMockInput
}
type metricMockInput struct {
	Name       string
	Unit       azmetrics.MetricUnit
	TimeSeries []azmetrics.TimeSeriesElement
}

// newQueryResourcesResponseMockData is a helper function to create an azmetrics list response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose azmetrics.QueryResourcesResponse.
func newQueryResourcesResponseMockData(inputSlice []queryResourceMockInput) azmetrics.QueryResourcesResponse {
	result := azmetrics.MetricResults{
		Values: make([]azmetrics.MetricData, len(inputSlice)),
	}
	for i, input := range inputSlice {
		values := make([]azmetrics.Metric, len(input.Metrics))
		for j, inputMetric := range input.Metrics {
			values[j] = azmetrics.Metric{
				Name: &azmetrics.LocalizableString{
					Value: to.Ptr(inputMetric.Name),
				},
				Unit:       to.Ptr(inputMetric.Unit),
				TimeSeries: inputMetric.TimeSeries,
			}
		}
		result.Values[i] = azmetrics.MetricData{
			ResourceID: to.Ptr(input.ResourceID),
			Values:     values,
		}
	}
	return azmetrics.QueryResourcesResponse{
		MetricResults: result,
	}
}

type metricsClientListResponseMockInput struct {
	Name       string
	Unit       armmonitor.Unit
	TimeSeries []*armmonitor.TimeSeriesElement
}

// newMetricsClientListResponseMockData is a helper function to create an armmonitor metrics list response.
// It is used ONLY for testing and ONLY to simplify the construction of mock data. Don't put intelligence in here.
// Use it for readability of the tests to avoid using verbose armmonitor.MetricsClientListResponse.
func newMetricsClientListResponseMockData(inputMap map[string]map[string][]metricsClientListResponseMockInput) map[string]map[string]armmonitor.MetricsClientListResponse {
	result := make(map[string]map[string]armmonitor.MetricsClientListResponse)
	for resourceID, inputMap2 := range inputMap {
		result2 := make(map[string]armmonitor.MetricsClientListResponse)
		for metricNames, inputSlice := range inputMap2 {
			result3 := armmonitor.MetricsClientListResponse{
				Response: armmonitor.Response{
					Value: make([]*armmonitor.Metric, len(inputSlice)),
				},
			}

			for i, input := range inputSlice {
				result3.Value[i] = &armmonitor.Metric{
					Name: &armmonitor.LocalizableString{
						Value: &input.Name,
					},
					Unit:       &input.Unit,
					Timeseries: input.TimeSeries,
				}
			}
			result2[metricNames] = result3
		}
		result[resourceID] = result2
	}
	return result
}
