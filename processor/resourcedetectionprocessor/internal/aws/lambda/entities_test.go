// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

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
		"cloud.provider": "aws", "cloud.platform": "aws_lambda", "cloud.region": "eu-west-1",
		"faas.name": "fn", "faas.version": "$LATEST", "faas.instance": "2026/11/01/[$LATEST]abc",
	}))
	ents := d.Entities(res)
	require.Len(t, ents, 2)
	// No account is available, so the cloud entity is not declared, but the region is.
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "cloud.region",
		IDKeys: []string{"cloud.region"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud",
	}, ents[0])
	assert.Equal(t, internal.DetectedEntity{
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "faas",
		IDKeys: []string{"faas.name"}, DescriptionKeys: []string{"faas.version", "faas.instance"},
		IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region",
	}, ents[1])

	assert.Empty(t, d.Entities(pcommon.NewResource()))
}
