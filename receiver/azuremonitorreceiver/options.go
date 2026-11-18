// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
)

type ClientOptionsResolver interface {
	GetArmResourceClientOptions(subscriptionID string) *arm.ClientOptions
	GetArmSubscriptionsClientOptions() *arm.ClientOptions
	GetArmMonitorClientOptions() *arm.ClientOptions
	GetAzMetricsClientOptions() *azmetrics.ClientOptions
}

type clientOptionsResolver struct {
	cloud cloud.Configuration
}

// newClientOptionsResolver creates a resolver that will always return the same options.
// Unlike in the tests where there will be one option by API mock, here we don't need different options for each client.
// Note the fact that it recreates the options each time. It's because the options are mutable, they can be modified by the client ctor.
func newClientOptionsResolver(cloudStr string) ClientOptionsResolver {
	var cloudToUse cloud.Configuration
	switch cloudStr {
	case azureGovernmentCloud:
		cloudToUse = cloud.AzureGovernment
	case azureChinaCloud:
		cloudToUse = cloud.AzureChina
	default:
		cloudToUse = cloud.AzurePublic
	}
	return &clientOptionsResolver{cloud: cloudToUse}
}

func (r *clientOptionsResolver) getClientOptions() azcore.ClientOptions {
	return azcore.ClientOptions{
		Cloud: r.cloud,
	}
}

func (r *clientOptionsResolver) GetArmResourceClientOptions(_ string) *arm.ClientOptions {
	return &arm.ClientOptions{
		ClientOptions: r.getClientOptions(),
	}
}

func (r *clientOptionsResolver) GetArmSubscriptionsClientOptions() *arm.ClientOptions {
	return &arm.ClientOptions{
		ClientOptions: r.getClientOptions(),
	}
}

func (r *clientOptionsResolver) GetArmMonitorClientOptions() *arm.ClientOptions {
	return &arm.ClientOptions{
		ClientOptions: r.getClientOptions(),
	}
}

func (r *clientOptionsResolver) GetAzMetricsClientOptions() *azmetrics.ClientOptions {
	options := r.getClientOptions()
	// Provides the standard retry codes and adds 529, which is used for backpressure by
	// the Batch API. See the Batch API Migration Guide
	// https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/migrate-to-batch-api?tabs=individual-response#529-throttling-errors
	options.Retry.StatusCodes = []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529, // Azure Monitor's throttling status for Batch API
	}
	return &azmetrics.ClientOptions{
		ClientOptions: options,
	}
}
