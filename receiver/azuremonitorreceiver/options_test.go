// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	armmonitorfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v2"
	armresourcesfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v2/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	armsubscriptionsfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions/fake"
)

func TestAzureScraperClientOptions(t *testing.T) {
	type fields struct {
		Cloud string
	}
	tests := []struct {
		name   string
		fields fields
		want   *arm.ClientOptions
	}{
		{
			name: "AzureCloud_options",
			fields: fields{
				Cloud: azureCloud,
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
				Cloud: azureGovernmentCloud,
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
				Cloud: azureChinaCloud,
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
			s := newClientOptionsResolver(tt.fields.Cloud)
			if got := s.GetArmResourceClientOptions("anything"); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArmClientOptions() = %v, want %v", got, tt.want)
			}
			if got := s.GetArmSubscriptionsClientOptions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArmClientOptions() = %v, want %v", got, tt.want)
			}
			if got := s.GetArmMonitorClientOptions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArmClientOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockClientOptionsResolver struct {
	armResourcesClientOptions     map[string]*arm.ClientOptions
	armSubscriptionsClientOptions *arm.ClientOptions
	armMonitorClientOptions       *arm.ClientOptions
}

// newMockClientOptionsResolver is an options resolver that will generate mocking client options for each Azure API.
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
) ClientOptionsResolver {
	// Init resources client options from resources mock data
	armResourcesClientOptions := make(map[string]*arm.ClientOptions)
	for subID, pages := range resources {
		resourceServer := armresourcesfake.Server{
			NewListPager: newMockResourcesListPager(pages),
		}
		armResourcesClientOptions[subID] = &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
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
			Transport: armmonitorfake.NewServerFactoryTransport(&armMonitorServerFactory),
		},
	}

	return &mockClientOptionsResolver{
		armResourcesClientOptions:     armResourcesClientOptions,
		armSubscriptionsClientOptions: armSubscriptionsClientOptions,
		armMonitorClientOptions:       armMonitorClientOptions,
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
