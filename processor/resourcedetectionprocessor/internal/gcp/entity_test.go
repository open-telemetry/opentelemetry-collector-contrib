// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp

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

	t.Run("gce cloud account", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderGCP.Value.AsString())
		res.Attributes().PutStr(string(conventions.CloudAccountIDKey), "project-1")
		res.Attributes().PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformGCPComputeEngine.Value.AsString())

		entities := d.EntityRefs(res)
		require.Len(t, entities, 1)
		assert.Equal(t, internal.EntityTypeCloudAccount, entities[0].Type)
		assert.Equal(t, []string{string(conventions.CloudProviderKey), string(conventions.CloudAccountIDKey)}, entities[0].IDKeys)
		assert.Equal(t, []string{string(conventions.CloudPlatformKey)}, entities[0].DescriptionKeys)
		assert.Empty(t, entities[0].IDContextTypeCandidates)
	})

	t.Run("gke cluster scoped to cloud account", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderGCP.Value.AsString())
		res.Attributes().PutStr(string(conventions.CloudAccountIDKey), "project-1")
		res.Attributes().PutStr(string(conventions.K8SClusterNameKey), "cluster-1")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 2)
		assert.Equal(t, internal.EntityTypeK8sCluster, entities[1].Type)
		assert.Equal(t, []string{string(conventions.K8SClusterNameKey)}, entities[1].IDKeys)
		assert.Equal(t, []string{internal.EntityTypeCloudAccount}, entities[1].IDContextTypeCandidates)
	})

	t.Run("no attributes", func(t *testing.T) {
		assert.Empty(t, d.EntityRefs(pcommon.NewResource()))
	})

	t.Run("missing cloud provider", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.CloudAccountIDKey), "project-1")
		assert.Empty(t, d.EntityRefs(res))
	})
}
