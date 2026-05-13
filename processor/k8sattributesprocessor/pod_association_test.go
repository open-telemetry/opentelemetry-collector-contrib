// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func TestExtractPodIDSkipsNonIPHostNameAssociation(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("host.name", "k8s-node-1")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: "host.name",
			}},
		},
	}

	pid := extractPodID(t.Context(), attrs, associations)
	assert.False(t, pid.IsNotEmpty())
}

func TestExtractPodIDFallsBackWhenHostNameIsNotIP(t *testing.T) {
	ctx := client.NewContext(t.Context(), client.Info{
		Addr: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 4317},
	})
	attrs := pcommon.NewMap()
	attrs.PutStr("host.name", "worker-node")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: "host.name",
			}},
		},
		{
			Sources: []kube.AssociationSource{{
				From: kube.ConnectionSource,
			}},
		},
	}

	pid := extractPodID(ctx, attrs, associations)
	require.True(t, pid.IsNotEmpty())
	assert.Equal(t, kube.ConnectionSource, pid[0].Source.From)
	assert.Equal(t, "1.2.3.4", pid[0].Value)
}

func TestExtractPodIDKeepsHostNameWhenValueIsIP(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("host.name", "10.1.2.3")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: "host.name",
			}},
		},
	}

	pid := extractPodID(t.Context(), attrs, associations)
	require.True(t, pid.IsNotEmpty())
	assert.Equal(t, kube.ResourceSource, pid[0].Source.From)
	assert.Equal(t, "host.name", pid[0].Source.Name)
	assert.Equal(t, "10.1.2.3", pid[0].Value)
}

func TestBuildPodIdentifierString(t *testing.T) {
	tests := []struct {
		name     string
		id       kube.PodIdentifier
		expected string
	}{
		{
			name:     "empty identifier returns empty string",
			id:       kube.PodIdentifier{},
			expected: "",
		},
		{
			name: "connection source omits actual IP value",
			id: kube.PodIdentifier{
				kube.PodIdentifierAttributeFromConnection("192.168.1.42"),
			},
			expected: "connection",
		},
		{
			name: "resource_attribute source uses attribute name not value",
			id: kube.PodIdentifier{
				kube.PodIdentifierAttributeFromResourceAttribute("k8s.pod.uid", "some-uid-value"),
			},
			expected: "resource_attribute/k8s.pod.uid",
		},
		{
			name: "resource_attribute with k8s.pod.ip",
			id: kube.PodIdentifier{
				kube.PodIdentifierAttributeFromResourceAttribute("k8s.pod.ip", "10.0.0.5"),
			},
			expected: "resource_attribute/k8s.pod.ip",
		},
		{
			name: "multi-source association joins with plus",
			id: kube.PodIdentifier{
				kube.PodIdentifierAttributeFromResourceAttribute("k8s.pod.uid", "uid-abc"),
				kube.PodIdentifierAttributeFromResourceAttribute("container.id", "container-xyz"),
			},
			expected: "resource_attribute/k8s.pod.uid+resource_attribute/container.id",
		},
		{
			name: "two different source types joined",
			id: kube.PodIdentifier{
				kube.PodIdentifierAttributeFromConnection("10.0.0.1"),
				kube.PodIdentifierAttributeFromResourceAttribute("k8s.pod.ip", "10.0.0.1"),
			},
			expected: "connection+resource_attribute/k8s.pod.ip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildPodIdentifierString(tt.id))
		})
	}
}
