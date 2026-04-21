// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

func TestExtractNodeIDByNodeName(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("k8s.node.name", "worker-1")

	associations := []kube.Association{
		{
			Sources: []kube.AssociationSource{
				{
					From: kube.ResourceSource,
					Name: "k8s.node.name",
				},
			},
		},
	}

	pid := extractNodeID(attrs, associations)
	require.True(t, pid.IsNotEmpty())
	assert.Equal(t, "worker-1", pid[0].Value)
}
