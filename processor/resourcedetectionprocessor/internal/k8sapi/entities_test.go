// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapi

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
	const schema = "https://opentelemetry.io/schemas/1.43.0"

	t.Run("uids are global", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{"k8s.node.uid": "nu", "k8s.node.name": "n", "k8s.cluster.uid": "cu", "host.id": "m"}))
		assert.Equal(t, []internal.DetectedEntity{
			{SchemaURL: schema, Type: "k8s.node", IDKeys: []string{"k8s.node.uid"}, DescriptionKeys: []string{"k8s.node.name"}, IDScope: xentity.IdScopeGlobal},
			{SchemaURL: schema, Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.uid"}, IDScope: xentity.IdScopeGlobal},
			{SchemaURL: schema, Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal},
		}, d.Entities(res))
	})
	t.Run("node name is local within the cluster, which need not be present", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr("k8s.node.name", "n")
		assert.Equal(t, []internal.DetectedEntity{
			{SchemaURL: schema, Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"},
		}, d.Entities(res))
	})
	assert.Empty(t, d.Entities(pcommon.NewResource()))
}
