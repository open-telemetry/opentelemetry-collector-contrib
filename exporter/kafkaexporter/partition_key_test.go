// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func TestResourceAttributeHash(t *testing.T) {
	build := func(kv map[string]string) pcommon.Map {
		m := pcommon.NewMap()
		for k, v := range kv {
			m.PutStr(k, v)
		}
		return m
	}

	t.Run("empty_keys_hashes_full_map", func(t *testing.T) {
		attrs := build(map[string]string{"customAttr": "12345", "service.name": "svc-a"})
		got := resourceAttributeHash(attrs, nil)
		want := pdatautil.MapHash(attrs)
		assert.Equal(t, want, got)
	})

	t.Run("subset_ignores_other_attrs", func(t *testing.T) {
		// Same customAttr, different service.name → same hash when subset = customAttr only.
		a := build(map[string]string{"customAttr": "12345", "service.name": "svc-a"})
		b := build(map[string]string{"customAttr": "12345", "service.name": "svc-b"})
		assert.Equal(t,
			resourceAttributeHash(a, []string{"customAttr"}),
			resourceAttributeHash(b, []string{"customAttr"}),
			"resources sharing the listed subset must hash to the same key")
	})

	t.Run("subset_distinguishes_different_values", func(t *testing.T) {
		a := build(map[string]string{"customAttr": "12345", "service.name": "svc-a"})
		c := build(map[string]string{"customAttr": "67890", "service.name": "svc-a"})
		assert.NotEqual(t,
			resourceAttributeHash(a, []string{"customAttr"}),
			resourceAttributeHash(c, []string{"customAttr"}),
			"different listed values must hash to different keys")
	})

	t.Run("subset_order_does_not_matter", func(t *testing.T) {
		attrs := build(map[string]string{"a": "1", "b": "2", "c": "3"})
		assert.Equal(t,
			resourceAttributeHash(attrs, []string{"a", "b"}),
			resourceAttributeHash(attrs, []string{"b", "a"}),
			"hash must be insensitive to key order in the configured list")
	})

	t.Run("missing_subset_keys_are_skipped", func(t *testing.T) {
		// Both resources lack "tenant.id"; their subset map is empty for that key,
		// so the hash collapses to whatever else is present (here: nothing).
		a := build(map[string]string{"service.name": "svc-a"})
		b := build(map[string]string{"service.name": "svc-b"})
		assert.Equal(t,
			resourceAttributeHash(a, []string{"tenant.id"}),
			resourceAttributeHash(b, []string{"tenant.id"}),
			"resources missing all subset keys must produce the same (empty-subset) hash")
	})

	t.Run("full_vs_subset_differ", func(t *testing.T) {
		attrs := build(map[string]string{"customAttr": "12345", "service.name": "svc-a"})
		full := resourceAttributeHash(attrs, nil)
		subset := resourceAttributeHash(attrs, []string{"customAttr"})
		assert.NotEqual(t, full, subset,
			"full-map hash and subset-map hash must differ for a non-trivial resource")
	})
}
