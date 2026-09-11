// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name    string
		base    map[string]any
		overlay map[string]any
		want    map[string]any
	}{
		{
			name:    "adds new key",
			base:    map[string]any{"a": float64(1)},
			overlay: map[string]any{"b": float64(2)},
			want:    map[string]any{"a": float64(1), "b": float64(2)},
		},
		{
			name:    "scalar overlay replaces scalar",
			base:    map[string]any{"a": float64(1)},
			overlay: map[string]any{"a": float64(9)},
			want:    map[string]any{"a": float64(9)},
		},
		{
			name:    "nested objects merge recursively",
			base:    map[string]any{"m": map[string]any{"x": float64(1), "y": float64(2)}},
			overlay: map[string]any{"m": map[string]any{"y": float64(3), "z": float64(4)}},
			want:    map[string]any{"m": map[string]any{"x": float64(1), "y": float64(3), "z": float64(4)}},
		},
		{
			name:    "array overlay replaces array wholesale",
			base:    map[string]any{"a": []any{float64(1), float64(2)}},
			overlay: map[string]any{"a": []any{float64(9)}},
			want:    map[string]any{"a": []any{float64(9)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepMerge(tt.base, tt.overlay)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMergeTemplateBody(t *testing.T) {
	base := `{
		"index_patterns": ["otel-v1-apm-span*"],
		"template": {
			"mappings": {
				"date_detection": false,
				"properties": {
					"traceId": {"type": "keyword"}
				}
			}
		}
	}`

	t.Run("overlay uplifts an attribute to a typed property", func(t *testing.T) {
		overlay := `{"template":{"mappings":{"properties":{"attributes":{"properties":{"http.status_code":{"type":"integer"}}}}}}}`
		merged, err := mergeTemplateBody(base, overlay)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(merged), &got))
		props := got["template"].(map[string]any)["mappings"].(map[string]any)["properties"].(map[string]any)
		// Base property is preserved.
		assert.Contains(t, props, "traceId")
		// Overlay property is added.
		attrs := props["attributes"].(map[string]any)["properties"].(map[string]any)
		assert.Equal(t, "integer", attrs["http.status_code"].(map[string]any)["type"])
		// Base scalar under mappings is preserved.
		assert.Equal(t, false, got["template"].(map[string]any)["mappings"].(map[string]any)["date_detection"])
	})

	t.Run("overlay can un-index attributes by disabling the object", func(t *testing.T) {
		overlay := `{"template":{"mappings":{"properties":{"attributes":{"type":"object","enabled":false}}}}}`
		merged, err := mergeTemplateBody(base, overlay)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(merged), &got))
		attrs := got["template"].(map[string]any)["mappings"].(map[string]any)["properties"].(map[string]any)["attributes"].(map[string]any)
		assert.Equal(t, false, attrs["enabled"])
	})

	t.Run("invalid overlay JSON returns error", func(t *testing.T) {
		_, err := mergeTemplateBody(base, `{not json}`)
		assert.ErrorContains(t, err, "custom index template file")
	})

	t.Run("invalid base JSON returns error", func(t *testing.T) {
		_, err := mergeTemplateBody(`{not json}`, `{}`)
		assert.ErrorContains(t, err, "built-in template")
	})
}
