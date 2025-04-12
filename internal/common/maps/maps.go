// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"

// MergeRawMaps merges n maps with a later map's keys overriding earlier maps.
func MergeRawMaps(maps ...map[string]any) map[string]any {
	ret := map[string]any{}

	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// MergeStringMaps merges n maps with a later map's keys overriding earlier maps.
func MergeStringMaps(maps ...map[string]string) map[string]string {
	ret := map[string]string{}

	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}

	return ret
}

// CloneStringMap makes a shallow copy of a map[string]string.
func CloneStringMap(m map[string]string) map[string]string {
	m2 := make(map[string]string, len(m))
	for k, v := range m {
		m2[k] = v
	}
	return m2
}
