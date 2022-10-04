package k8sobjectreceiver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
)

type MockDiscovery struct {
	fakeDiscovery.FakeDiscovery
}

func (c *MockDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name: "pods",
					Kind: "Pods",
				},
				{
					Name: "events",
					Kind: "Events",
				},
			},
		},
	}, nil
}

func getMockDiscoveryClient() (discovery.ServerResourcesInterface, error) {
	return &MockDiscovery{}, nil
}
