// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks

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
	res := pcommon.NewResource()
	require.NoError(t, res.Attributes().FromRaw(map[string]any{
		"cloud.provider": "aws", "cloud.platform": "aws_eks", "cloud.account.id": "123", "cloud.region": "us-west-2", "cloud.availability_zone": "us-west-2b",
		"host.id": "i-abc", "host.name": "ip-1", "k8s.cluster.name": "prod",
	}))
	ents := d.Entities(res)
	require.Len(t, ents, 5)
	types := []string{}
	for _, e := range ents {
		types = append(types, e.Type)
	}
	assert.Equal(t, []string{"cloud", "cloud.region", "cloud.availability_zone", "host", "k8s.cluster"}, types)
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "k8s.cluster",
		IDKeys: []string{"k8s.cluster.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region",
	}, ents[4])
	assert.Equal(t, "cloud.region", ents[3].IDScopeEntityType)

	assert.Empty(t, d.Entities(pcommon.NewResource()))
}
