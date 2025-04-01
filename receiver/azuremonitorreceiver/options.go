// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

type ClientOptionsResolver interface {
	GetArmResourceClientOptions(subscriptionID string) *arm.ClientOptions
	GetArmSubscriptionsClientOptions() *arm.ClientOptions
	GetArmMonitorClientOptions() *arm.ClientOptions
}

type clientOptionsResolver struct {
	options *arm.ClientOptions
}

// newClientOptionsResolver creates a resolver that will always return the same options.
// Unlike in the tests where there will be one option by API mock, here we don't need different options for each client.
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
	return &clientOptionsResolver{options: &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloudToUse,
		},
	}}
}

func (r *clientOptionsResolver) GetArmResourceClientOptions(_ string) *arm.ClientOptions {
	return r.options
}

func (r *clientOptionsResolver) GetArmSubscriptionsClientOptions() *arm.ClientOptions {
	return r.options
}

func (r *clientOptionsResolver) GetArmMonitorClientOptions() *arm.ClientOptions {
	return r.options
}
