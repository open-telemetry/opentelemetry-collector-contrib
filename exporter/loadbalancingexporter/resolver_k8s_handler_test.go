// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestConvertToEndpoints(tst *testing.T) {
	hostname1 := "pod-1"
	hostname2 := "pod-2"
	hostname4 := "pod-4"
	hostname5 := "pod-5"

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
	// A slice mixing an explicitly ready, an explicitly not-ready (terminating), and a
	// nil-readiness ("unknown", treated as ready per the EndpointSlice API) endpoint.
	endpointsMixedReadiness := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-4",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.104"},
				Hostname:  &hostname4,
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				Addresses: []string{"192.168.10.105"},
				Hostname:  &hostname5,
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(false),
					Serving:     ptr.To(true),
					Terminating: ptr.To(true),
				},
			},
			{
				Addresses: []string{"192.168.10.106"},
			},
		},
	}
	// A slice whose only endpoint is not ready and also misses the hostname: the
	// readiness filter must skip it before the hostname validation can fail.
	endpointsNotReadyNoHostname := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoints-5",
			Namespace: "test-namespace",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.10.107"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(false),
				},
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
			name:              "not-ready endpoint excluded, nil readiness kept (IPs)",
			returnNames:       false,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpointsMixedReadiness},
			expectedEndpoints: map[string]bool{"192.168.10.104": true, "192.168.10.106": true},
			wantOk:            true,
		},
		{
			name:              "not-ready endpoint excluded (hostnames)",
			returnNames:       true,
			includedEndpoints: []*discoveryv1.EndpointSlice{endpoints1, endpointsNotReadyNoHostname},
			expectedEndpoints: map[string]bool{"pod-1": true},
			wantOk:            true,
		},
	}

	for _, tt := range tests {
		tst.Run(tt.name, func(tst *testing.T) {
			ok, res := convertToEndpoints(zap.NewNop(), tt.returnNames, tt.includedEndpoints...)
			assert.Equal(tst, tt.expectedEndpoints, res)
			assert.Equal(tst, tt.wantOk, ok)
		})
	}
}
