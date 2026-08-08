// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

type kubernetesClients struct {
	dynamic   dynamic.Interface
	metadata  metadata.Interface
	discovery discovery.ServerResourcesInterface
}

func newKubernetesClients(restConfig *rest.Config) (kubernetesClients, error) {
	restConfig = rest.CopyConfig(restConfig)
	if restConfig.RateLimiter == nil {
		qps := restConfig.QPS
		if qps == 0 {
			qps = rest.DefaultQPS
		}
		burst := restConfig.Burst
		if burst == 0 {
			burst = rest.DefaultBurst
		}
		restConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
	}

	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return kubernetesClients{}, err
	}
	dynamicClient, err := dynamic.NewForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return kubernetesClients{}, err
	}
	metadataClient, err := metadata.NewForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return kubernetesClients{}, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(restConfig, httpClient)
	if err != nil {
		return kubernetesClients{}, err
	}

	return kubernetesClients{
		dynamic:   dynamicClient,
		metadata:  metadataClient,
		discovery: discoveryClient,
	}, nil
}
