// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
)

// fakeLister is a test double for EndpointSliceLister.
type fakeLister struct {
	slices            []discoveryv1.EndpointSlice
	err               error
	capturedNamespace string
	capturedSelector  string
}

func (f *fakeLister) ListEndpointSlices(_ context.Context, namespace, labelSelector string) ([]discoveryv1.EndpointSlice, error) {
	f.capturedNamespace = namespace
	f.capturedSelector = labelSelector
	return f.slices, f.err
}

// makeSampleSlices returns a minimal but realistic EndpointSlice list.
func makeSampleSlices() []discoveryv1.EndpointSlice {
	tcp := corev1.ProtocolTCP
	port4317 := int32(4317)
	port4318 := int32(4318)
	return []discoveryv1.EndpointSlice{
		{
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.1"},
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "otelcol-pod-abc"},
				},
				{
					Addresses: []string{"10.0.0.2"},
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "otelcol-pod-def"},
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{Port: &port4317, Protocol: &tcp},
				{Port: &port4318, Protocol: &tcp},
			},
		},
	}
}

func TestFetchGatewayInfo_Success(t *testing.T) {
	lister := &fakeLister{slices: makeSampleSlices()}
	info, err := FetchGatewayInfo(context.Background(), "my-service", lister)
	require.NoError(t, err)

	assert.Equal(t, "my-service", info.Service)
	assert.Equal(t, "IPv4", info.AddressType)
	assert.Equal(t, []string{"4317/TCP", "4318/TCP"}, info.Ports)
	assert.Equal(t, []string{"otelcol-pod-abc", "otelcol-pod-def"}, info.Pods)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, info.Addresses)
}

func TestFetchGatewayInfo_WithNamespace(t *testing.T) {
	lister := &fakeLister{slices: makeSampleSlices()}
	info, err := FetchGatewayInfo(context.Background(), "monitoring/my-service", lister)
	require.NoError(t, err)

	assert.Equal(t, "my-service", info.Service)
	assert.Equal(t, "monitoring", info.Namespace)

	// Verify the namespace and label selector were passed correctly.
	assert.Equal(t, "monitoring", lister.capturedNamespace)
	assert.Equal(t, "kubernetes.io/service-name=my-service", lister.capturedSelector)
}

func TestFetchGatewayInfo_WithoutNamespace(t *testing.T) {
	lister := &fakeLister{slices: makeSampleSlices()}
	// Without a namespace, FetchGatewayInfo tries in-cluster namespace detection,
	// which will fail outside a cluster, so namespace stays "".
	_, err := FetchGatewayInfo(context.Background(), "my-service", lister)
	require.NoError(t, err)

	// Label selector should still reference the service.
	assert.Equal(t, "kubernetes.io/service-name=my-service", lister.capturedSelector)
}

func TestFetchGatewayInfo_ListerError(t *testing.T) {
	lister := &fakeLister{err: fmt.Errorf("connection refused")}
	_, err := FetchGatewayInfo(context.Background(), "my-service", lister)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list endpoint slices")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestFetchGatewayInfo_EmptyItemList(t *testing.T) {
	lister := &fakeLister{slices: []discoveryv1.EndpointSlice{}}
	info, err := FetchGatewayInfo(context.Background(), "my-service", lister)
	require.NoError(t, err)

	assert.Equal(t, "my-service", info.Service)
	assert.Empty(t, info.Ports)
	assert.Empty(t, info.Pods)
	assert.Empty(t, info.Addresses)
}

func TestFetchGatewayInfo_MultipleSlices(t *testing.T) {
	// Two slices → deduplication of ports, union of pods/addresses.
	tcp := corev1.ProtocolTCP
	port4317 := int32(4317)
	port4318 := int32(4318)
	lister := &fakeLister{
		slices: []discoveryv1.EndpointSlice{
			{
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{"10.0.0.1"}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-a"}},
				},
				Ports: []discoveryv1.EndpointPort{
					{Port: &port4317, Protocol: &tcp},
				},
			},
			{
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{"10.0.0.2"}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-b"}},
				},
				Ports: []discoveryv1.EndpointPort{
					{Port: &port4317, Protocol: &tcp},
					{Port: &port4318, Protocol: &tcp},
				},
			},
		},
	}
	info, err := FetchGatewayInfo(context.Background(), "my-svc", lister)
	require.NoError(t, err)

	assert.Equal(t, []string{"4317/TCP", "4318/TCP"}, info.Ports)
	assert.Equal(t, []string{"pod-a", "pod-b"}, info.Pods)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, info.Addresses)
}

func TestFetchGatewayInfo_NilPortIgnored(t *testing.T) {
	// A port entry with nil port should be silently skipped.
	lister := &fakeLister{
		slices: []discoveryv1.EndpointSlice{
			{
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   []discoveryv1.Endpoint{},
				Ports:       []discoveryv1.EndpointPort{{Port: nil}},
			},
		},
	}
	info, err := FetchGatewayInfo(context.Background(), "svc", lister)
	require.NoError(t, err)
	assert.Empty(t, info.Ports)
}

func TestParseServiceName(t *testing.T) {
	tests := []struct {
		input         string
		wantName      string
		wantNamespace string
	}{
		{"my-service", "my-service", ""},
		{"default/my-service", "my-service", "default"},
		{"monitoring/otelcol-gateway", "otelcol-gateway", "monitoring"},
	}
	for _, tt := range tests {
		name, ns := parseServiceName(tt.input)
		assert.Equal(t, tt.wantName, name, "input: %s", tt.input)
		assert.Equal(t, tt.wantNamespace, ns, "input: %s", tt.input)
	}
}
