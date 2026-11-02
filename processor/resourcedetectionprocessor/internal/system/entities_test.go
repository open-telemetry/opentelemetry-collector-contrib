// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

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

	t.Run("host.id is a machine id: global", func(t *testing.T) {
		res := pcommon.NewResource()
		require.NoError(t, res.Attributes().FromRaw(map[string]any{"host.id": "m1", "host.name": "h", "os.type": "linux", "host.arch": "arm64"}))
		ents := d.Entities(res)
		require.Len(t, ents, 1)
		assert.Equal(t, internal.DetectedEntity{
			SchemaURL: "https://opentelemetry.io/schemas/1.43.0", Type: "host",
			IDKeys: []string{"host.id"}, DescriptionKeys: []string{"host.arch", "host.name", "os.type"},
			IDScope: xentity.IdScopeGlobal,
		}, ents[0])
	})
	t.Run("host.name only: unspecified scope", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr("host.name", "h")
		ents := d.Entities(res)
		require.Len(t, ents, 1)
		assert.Equal(t, []string{"host.name"}, ents[0].IDKeys)
		assert.Equal(t, xentity.IdScopeUnspecified, ents[0].IDScope)
		assert.Empty(t, ents[0].IDScopeEntityType)
	})
	t.Run("nothing", func(t *testing.T) {
		assert.Empty(t, d.Entities(pcommon.NewResource()))
	})
}
