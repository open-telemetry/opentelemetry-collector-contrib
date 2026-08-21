// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertToEndpoints(tst *testing.T) {
	hostname1 := "pod-1"
	hostname2 := "pod-2"

	// Create dummy EndpointSlice objects
	endpoints1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-1",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.101"},
				Hostname:  &hostname1,
			},
		},
	}
	endpoints2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-2",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.102"},
				Hostname:  &hostname2,
			},
		},
	}
	endpoints3 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-3",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.103"},
			},
		},
	}
	notReady := false
	ready := true
	hostname4 := "pod-4"
	endpointsNotReady := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-4",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"192.168.10.104"},
				Hostname:   &hostname4,
				Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
			},
		},
	}
	hostname5 := "pod-5"
	endpointsExplicitlyReady := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-5",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"192.168.10.105"},
				Hostname:   &hostname5,
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			},
		},
	}

	tests := []struct {
		name              string
		returnNames       bool
		includedEndpoints []*discoveryv1.EndpointSlice
		expectedEndpoints map[string]bool
		wantOk            bool
	}{
		{
			name:              "return hostnames",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints2},
			expectedEndpoints: map[string]bool{"pod-1": true, "pod-2": true},
			wantOk:            true,
		},
		{
			name:              "return IPs",
			returnNames:       false,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints2, endpoints3},
			expectedEndpoints: map[string]bool{"192.168.10.101": true, "192.168.10.102": true, "192.168.10.103": true},
			wantOk:            true,
		},
		{
			// An endpoint missing its hostname is skipped rather than discarding
			// the whole slice: the endpoints that do have hostnames are still
			// returned, and ok is false to signal that some were skipped.
			name:              "missing hostname is skipped, others returned",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints3},
			expectedEndpoints: map[string]bool{"pod-1": true},
			wantOk:            false,
		},
		{
			// Ready == false is an explicit exclusion, not a missing-data case,
			// so ok stays true even though the endpoint is skipped.
			name:              "not ready endpoint is excluded",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpointsNotReady},
			expectedEndpoints: map[string]bool{"pod-1": true},
			wantOk:            true,
		},
		{
			// A nil Ready condition (endpoints1/2/3 never set Conditions) and an
			// explicit Ready == true both include the endpoint.
			name:              "nil and explicit-true ready conditions are both included",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpointsExplicitlyReady},
			expectedEndpoints: map[string]bool{"pod-1": true, "pod-5": true},
			wantOk:            true,
		},
	}

	for _, tt := range tests {
		tst.Run(tt.name, func(tst *testing.T) {
			ok, res := convertToEndpoints(tt.returnNames, tt.includedEndpoints...)
			assert.Equal(tst, tt.expectedEndpoints, res)
			assert.Equal(tst, tt.wantOk, ok)
		})
	}
}
