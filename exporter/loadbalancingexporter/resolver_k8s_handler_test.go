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

	tests := []struct {
		name              string
		returnNames       bool
		includedEndpoints []*discoveryv1.EndpointSlice
		expectedEndpoints map[string]bool
		wantNil           bool
	}{
		{
			name:              "return hostnames",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints2},
			expectedEndpoints: map[string]bool{"pod-1": true, "pod-2": true},
			wantNil:           false,
		},
		{
			name:              "return IPs",
			returnNames:       false,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints2, endpoints3},
			expectedEndpoints: map[string]bool{"192.168.10.101": true, "192.168.10.102": true, "192.168.10.103": true},
			wantNil:           false,
		},
		{
			name:              "missing hostname",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpoints3},
			expectedEndpoints: nil,
			wantNil:           true,
		},
	}

	for _, tt := range tests {
		tst.Run(tt.name, func(tst *testing.T) {
			ok, res := convertToEndpoints(tt.returnNames, tt.includedEndpoints...)
			if tt.wantNil {
				assert.Nil(tst, res)
			} else {
				assert.Equal(tst, tt.expectedEndpoints, res)
			}
			assert.Equal(tst, !tt.wantNil, ok)
		})
	}
}
