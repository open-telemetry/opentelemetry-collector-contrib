// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestEntityRefs(t *testing.T) {
	d := &detector{}

	t.Run("node uid present", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.K8SNodeUIDKey), "node-uid-1")
		res.Attributes().PutStr(string(conventions.K8SNodeNameKey), "node-1")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 1)
		assert.Equal(t, internal.EntityTypeK8sNode, entities[0].Type)
		assert.Equal(t, []string{string(conventions.K8SNodeUIDKey)}, entities[0].IDKeys)
		assert.Equal(t, []string{string(conventions.K8SNodeNameKey)}, entities[0].DescriptionKeys)
		assert.Empty(t, entities[0].IDContextTypeCandidates)
	})

	t.Run("node name only", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.K8SNodeNameKey), "node-1")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 1)
		assert.Equal(t, []string{string(conventions.K8SNodeNameKey)}, entities[0].IDKeys)
		assert.Equal(t, []string{internal.EntityTypeK8sCluster}, entities[0].IDContextTypeCandidates)
	})

	t.Run("cluster uid present", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.K8SNodeNameKey), "node-1")
		res.Attributes().PutStr(string(conventions.K8SClusterUIDKey), "cluster-uid-1")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 2)
		assert.Equal(t, internal.EntityTypeK8sCluster, entities[1].Type)
		assert.Equal(t, []string{string(conventions.K8SClusterUIDKey)}, entities[1].IDKeys)
		assert.Empty(t, entities[1].IDContextTypeCandidates)
	})
}
