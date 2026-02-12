// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDimensionUpdate_Hash(t *testing.T) {
	tests := []struct {
		name    string
		update1 *DimensionUpdate
		update2 *DimensionUpdate
		equal   bool
	}{
		{
			name: "identical_properties",
			update1: &DimensionUpdate{
				Properties: map[string]*string{
					"a": strPtr("1"),
					"b": strPtr("2"),
					"c": strPtr("3"),
				},
			},
			update2: &DimensionUpdate{
				Properties: map[string]*string{
					"c": strPtr("3"),
					"a": strPtr("1"),
					"b": strPtr("2"),
				},
			},
			equal: true,
		},
		{
			name: "different_property_values",
			update1: &DimensionUpdate{
				Properties: map[string]*string{"prop": strPtr("val1")},
			},
			update2: &DimensionUpdate{
				Properties: map[string]*string{"prop": strPtr("val2")},
			},
			equal: false,
		},
		{
			name: "nil_vs_non_nil_property",
			update1: &DimensionUpdate{
				Properties: map[string]*string{"prop": nil},
			},
			update2: &DimensionUpdate{
				Properties: map[string]*string{"prop": strPtr("value")},
			},
			equal: false,
		},
		{
			name: "identical_tags",
			update1: &DimensionUpdate{
				Tags: map[string]bool{"tag1": true, "tag2": false, "tag3": true},
			},
			update2: &DimensionUpdate{
				Tags: map[string]bool{"tag3": true, "tag1": true, "tag2": false},
			},
			equal: true,
		},
		{
			name: "different_tag_values",
			update1: &DimensionUpdate{
				Tags: map[string]bool{"tag": true},
			},
			update2: &DimensionUpdate{
				Tags: map[string]bool{"tag": false},
			},
			equal: false,
		},
		{
			name: "property_vs_tag_collision_resistance",
			update1: &DimensionUpdate{
				Properties: map[string]*string{"key": strPtr("value")},
			},
			update2: &DimensionUpdate{
				Tags: map[string]bool{"key": true},
			},
			equal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := tt.update1.Hash()
			hash2 := tt.update2.Hash()
			if tt.equal {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}
