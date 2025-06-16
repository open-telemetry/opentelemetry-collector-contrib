// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
)

type mockDiscovery struct {
	fakeDiscovery.FakeDiscovery
}

func (c *mockDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name: "pods",
					Kind: "Pod",
				},
				{
					Name: "events",
					Kind: "Event",
				},
			},
		},
		{
			GroupVersion: "events.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "events",
					Kind: "Event",
				},
			},
		},
		{
			GroupVersion: "group1/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "myresources",
					Kind: "MyResource",
				},
			},
		},
		{
			GroupVersion: "group2/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "myresources",
					Kind: "MyResource",
				},
			},
		},
	}, nil
}

func getMockDiscoveryClient() (discovery.ServerResourcesInterface, error) {
	return &mockDiscovery{}, nil
}
