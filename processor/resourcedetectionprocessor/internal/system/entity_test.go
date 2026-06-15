// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestEntityRefs(t *testing.T) {
	d := &Detector{}

	t.Run("host id", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.HostIDKey), "host-id-1")
		res.Attributes().PutStr(string(conventions.HostNameKey), "host-1")
		res.Attributes().PutStr(string(conventions.OSTypeKey), "linux")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 1)
		assert.Equal(t, internal.EntityTypeHost, entities[0].Type)
		assert.Equal(t, []string{string(conventions.HostIDKey)}, entities[0].IDKeys)
		assert.Equal(t, []string{string(conventions.HostNameKey), string(conventions.OSTypeKey)}, entities[0].DescriptionKeys)
		assert.Empty(t, entities[0].IDContextTypeCandidates)
	})

	t.Run("host name fallback", func(t *testing.T) {
		res := pcommon.NewResource()
		res.Attributes().PutStr(string(conventions.HostNameKey), "host-1")

		entities := d.EntityRefs(res)
		require.Len(t, entities, 1)
		assert.Equal(t, []string{string(conventions.HostNameKey)}, entities[0].IDKeys)
		assert.Empty(t, entities[0].IDContextTypeCandidates)
	})

	t.Run("no identity", func(t *testing.T) {
		assert.Empty(t, d.EntityRefs(pcommon.NewResource()))
	})
}
