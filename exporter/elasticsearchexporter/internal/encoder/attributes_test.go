// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func TestEncodeAttributes(t *testing.T) {
	t.Parallel()

	attributes := pcommon.NewMap()
	err := attributes.FromRaw(map[string]any{
		"s": "baz",
		"o": map[string]any{
			"sub_i": 19,
		},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		mappingMode mapping.Mode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: mapping.ModeRaw,
			want: func() objmodel.Document {
				return objmodel.DocumentFromAttributes(attributes)
			},
		},
		"none": {
			mappingMode: mapping.ModeNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
		"ecs": {
			mappingMode: mapping.ModeECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := objmodel.Document{}
			EncodeAttributes(test.mappingMode, &doc, attributes)
			require.Equal(t, test.want(), doc)
		})
	}
}
