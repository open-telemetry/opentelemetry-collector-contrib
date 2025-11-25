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
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func TestExtractPodIDSkipsNonIPHostNameAssociation(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr(string(conventions.HostNameKey), "k8s-node-1")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: string(conventions.HostNameKey),
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
	attrs.PutStr(string(conventions.HostNameKey), "worker-node")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: string(conventions.HostNameKey),
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
	attrs.PutStr(string(conventions.HostNameKey), "10.1.2.3")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{{
				From: kube.ResourceSource,
				Name: string(conventions.HostNameKey),
			}},
		},
	}

	pid := extractPodID(t.Context(), attrs, associations)
	require.True(t, pid.IsNotEmpty())
	assert.Equal(t, kube.ResourceSource, pid[0].Source.From)
	assert.Equal(t, string(conventions.HostNameKey), pid[0].Source.Name)
	assert.Equal(t, "10.1.2.3", pid[0].Value)
}
