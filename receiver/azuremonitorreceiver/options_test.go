// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
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

func TestAzureBatchAPIEndpointOptions(t *testing.T) {
	type fields struct {
		Cloud  string
		Region string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "AzureCloud_endpoint",
			fields: fields{
				Cloud:  azureCloud,
				Region: "eastus",
			},
			want: "https://eastus.metrics.monitor.azure.com",
		},
		{
			name: "AzureGovernmentCloud_endpoint",
			fields: fields{
				Cloud:  azureGovernmentCloud,
				Region: "usgovvirginia",
			},
			want: "https://usgovvirginia.metrics.monitor.azure.us",
		},
		{
			name: "AzureChinaCloud_endpoint",
			fields: fields{
				Cloud:  azureChinaCloud,
				Region: "chinaeast2",
			},
			want: "https://chinaeast2.metrics.monitor.azure.cn",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newClientOptionsResolver(tt.fields.Cloud)
			if got := s.GetBatchAPIEndpoint(tt.fields.Region); got != tt.want {
				t.Errorf("GetBatchAPIEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
