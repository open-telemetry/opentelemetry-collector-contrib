// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestEntities(t *testing.T) {
	d := &detector{}
	t.Run("gke", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{
			"cloud.provider": "gcp", "cloud.platform": "gcp_kubernetes_engine", "cloud.account.id": "proj", "cloud.region": "us-central1",
			"host.id": "123", "host.name": "gke-node", "k8s.cluster.name": "c1",
		}))
		ents := d.Entities(res)
		require.Len(t, ents, 4)
		assert.Equal(t, "cloud", ents[0].Type)
		assert.Equal(t, xentity.IdScopeGlobal, ents[0].IDScope)
		assert.Equal(t, "cloud.region", ents[1].Type)
		assert.Equal(t, internal.DetectedEntity{
			SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "host",
			IDKeys: []string{"host.id"}, DescriptionKeys: []string{"host.name"},
			IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud",
		}, ents[2])
		assert.Equal(t, internal.DetectedEntity{
			SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "k8s.cluster",
			IDKeys:  []string{"k8s.cluster.name"},
			IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region",
		}, ents[3])
	})
	t.Run("zonal gke cluster is scoped by the zone", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{
			"cloud.provider": "gcp", "cloud.account.id": "proj", "cloud.region": "us-central1", "cloud.availability_zone": "us-central1-a",
			"k8s.cluster.name": "c1",
		}))
		ents := d.Entities(res)
		require.Len(t, ents, 4)
		assert.Equal(t, "cloud.availability_zone", ents[2].Type)
		assert.Equal(t, "cloud", ents[2].IDScopeEntityType)
		assert.Equal(t, "k8s.cluster", ents[3].Type)
		assert.Equal(t, xentity.IdScopeLocal, ents[3].IDScope)
		assert.Equal(t, "cloud.availability_zone", ents[3].IDScopeEntityType)
	})
	t.Run("gce", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{
			"cloud.provider": "gcp", "cloud.platform": "gcp_compute_engine", "cloud.account.id": "proj",
			"host.id": "123", "host.type": "n2", "gcp.gce.instance.name": "vm", "gcp.gce.instance.hostname": "vm.internal",
		}))
		ents := d.Entities(res)
		require.Len(t, ents, 2)
		assert.Equal(t, []string{"host.type", "gcp.gce.instance.hostname", "gcp.gce.instance.name"}, ents[1].DescriptionKeys)
		assert.Equal(t, "cloud", ents[1].IDScopeEntityType)
	})
	t.Run("cloud run service is scoped by region", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{"cloud.provider": "gcp", "cloud.account.id": "proj", "cloud.region": "us-east1", "faas.name": "f", "faas.version": "3"}))
		ents := d.Entities(res)
		require.Len(t, ents, 3)
		assert.Equal(t, internal.DetectedEntity{
			SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "faas",
			IDKeys: []string{"faas.name"}, DescriptionKeys: []string{"faas.version"},
			IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region",
		}, ents[2])
	})
	assert.Empty(t, d.Entities(pcommon.NewResource()))
}
