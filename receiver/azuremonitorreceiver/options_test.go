// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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
					Retry: policy.RetryOptions{StatusCodes: retryStatusCodes()},
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
					Retry: policy.RetryOptions{StatusCodes: retryStatusCodes()},
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
					Retry: policy.RetryOptions{StatusCodes: retryStatusCodes()},
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

func TestRetryStatusCodesIncludeDefaultsAnd529(t *testing.T) {
	codes := retryStatusCodes()
	for _, code := range []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529,
	} {
		assert.Contains(t, codes, code)
	}
}

// statusSequenceTransport replies with the given status codes in order, then 200.
type statusSequenceTransport struct {
	statuses []int
	calls    int
}

func (s *statusSequenceTransport) Do(req *http.Request) (*http.Response, error) {
	status := http.StatusOK
	if s.calls < len(s.statuses) {
		status = s.statuses[s.calls]
	}
	s.calls++
	return &http.Response{StatusCode: status, Body: http.NoBody, Request: req}, nil
}

func TestClientOptionsRetryOnThrottlingStatus(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, 529} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			transport := &statusSequenceTransport{statuses: []int{status}}
			opts := newClientOptionsResolver(azureCloud).GetArmMonitorClientOptions()
			opts.Transport = transport
			opts.Retry.RetryDelay = time.Millisecond

			pipeline := runtime.NewPipeline("test", "v0.0.0", runtime.PipelineOptions{}, &opts.ClientOptions)
			req, err := runtime.NewRequest(t.Context(), http.MethodGet, "http://localhost/")
			require.NoError(t, err)

			resp, err := pipeline.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, 2, transport.calls)
		})
	}
}
