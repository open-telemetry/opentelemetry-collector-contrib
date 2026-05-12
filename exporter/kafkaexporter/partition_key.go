// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// resourceAttributeHash returns a deterministic hash of the given resource
// attribute map. When keys is empty, the full map is hashed (preserving the
// previous default behavior). When keys is non-empty, only the listed keys
// contribute to the hash; absent keys are treated as not present.
//
// MapHash already sorts entries by key, so the order of keys passed in here
// does not affect the output. Two resources whose listed-key values are equal
// — regardless of any other attributes — therefore produce the same hash.
func resourceAttributeHash(attrs pcommon.Map, keys []string) [16]byte {
	if len(keys) == 0 {
		return pdatautil.MapHash(attrs)
	}
	subset := pcommon.NewMap()
	subset.EnsureCapacity(len(keys))
	for _, k := range keys {
		if v, ok := attrs.Get(k); ok {
			v.CopyTo(subset.PutEmpty(k))
		}
	}
	return pdatautil.MapHash(subset)
}
