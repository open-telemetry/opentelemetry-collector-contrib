// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestConvertToEndpoints(tst *testing.T) {
	// Create dummy EndpointSlice objects
	endpointSlice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-1-abc123",
			Namespace: "test-namespace",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service-1",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.101"},
				Hostname:  ptr.To("pod-1"),
			},
		},
	}
	endpointSlice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-2-def456",
			Namespace: "test-namespace",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service-2",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.102"},
				Hostname:  ptr.To("pod-2"),
			},
		},
	}
	endpointSlice3 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-3-ghi789",
			Namespace: "test-namespace",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service-3",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
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
			includedEndpoints: []*discoveryv1.EndpointSlice{endpointSlice1, endpointSlice2},
			expectedEndpoints: map[string]bool{"pod-1": true, "pod-2": true},
			wantNil:           false,
		},
		{
			name:              "return IPs",
			returnNames:       false,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpointSlice1, endpointSlice2, endpointSlice3},
			expectedEndpoints: map[string]bool{"192.168.10.101": true, "192.168.10.102": true, "192.168.10.103": true},
			wantNil:           false,
		},
		{
			name:              "missing hostname",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpointSlice1, endpointSlice3},
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
