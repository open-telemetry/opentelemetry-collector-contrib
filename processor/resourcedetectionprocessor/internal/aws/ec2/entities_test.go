// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestEntities(t *testing.T) {
	d := &Detector{}
	res := pcommon.NewResource()
	require.NoError(t, res.Attributes().FromRaw(map[string]any{
		"cloud.provider": "aws", "cloud.platform": "aws_ec2", "cloud.region": "us-west-2", "cloud.account.id": "123", "cloud.availability_zone": "us-west-2a",
		"host.id": "i-abc", "host.image.id": "ami-1", "host.type": "m5.large", "host.name": "ip-1",
		"ec2.tag.team": "x", "ec2.tag.env": "y",
	}))
	ents := d.Entities(res)
	require.Len(t, ents, 4)
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "cloud",
		IDKeys: []string{"cloud.provider", "cloud.account.id"}, DescriptionKeys: []string{"cloud.platform"},
		IDScope: xentity.IdScopeGlobal,
	}, ents[0])
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "cloud.region",
		IDKeys: []string{"cloud.region"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud",
	}, ents[1])
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "cloud.availability_zone",
		IDKeys: []string{"cloud.availability_zone"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud",
	}, ents[2])
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "host",
		IDKeys: []string{"host.id"}, DescriptionKeys: []string{"host.name", "host.type", "host.image.id", "ec2.tag.env", "ec2.tag.team"},
		IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region",
	}, ents[3])

	t.Run("no region: host scope unspecified", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{"cloud.provider": "aws", "cloud.account.id": "123", "host.id": "i-abc"}))
		ents := d.Entities(res)
		require.Len(t, ents, 2)
		assert.Equal(t, xentity.IdScopeUnspecified, ents[1].IDScope)
		assert.Empty(t, ents[1].IDScopeEntityType)
	})
	assert.Empty(t, d.Entities(pcommon.NewResource()))
}
