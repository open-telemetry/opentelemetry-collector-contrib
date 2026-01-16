// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
)

func TestGetKey(t *testing.T) {
	for _, tc := range []struct {
		name         string
		metadataKeys []string
		metadata     map[string][]string
		expected     string
	}{
		{
			name:         "empty",
			metadataKeys: nil,
			metadata: map[string][]string{
				"key1": {"val1"},
			},
			expected: "",
		},
		{
			name:         "with_missing_key",
			metadataKeys: []string{"key404"},
			metadata: map[string][]string{
				"key1": {"val1"},
			},
			expected: "",
		},
		{
			name:         "with_key_in_metadata",
			metadataKeys: []string{"key1"},
			metadata: map[string][]string{
				"key1": {"val1"},
			},
			expected: "key1\x00val1",
		},
		{
			name:         "with_multiple_key_in_metadata",
			metadataKeys: []string{"key1", "key2"},
			metadata: map[string][]string{
				"key1": {"val1"},
				"key2": {"val2.1", "val2.2", "val2.3"},
			},
			expected: "key1\x00val1\x00key2\x00val2.1\x00val2.2\x00val2.3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := client.NewContext(t.Context(), client.Info{
				Metadata: client.NewMetadata(tc.metadata),
			})
			assert.Equal(t, tc.expected, metadataKeysPartitioner{keys: tc.metadataKeys}.GetKey(ctx, nil))
		})
	}
}

func BenchmarkGetKey(b *testing.B) {
	p := metadataKeysPartitioner{keys: []string{"key1", "key2"}}
	ctx := client.NewContext(b.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"key1": {"val1"},
			"key2": {"val2.1", "val2.2", "val2.3"},
		}),
	})

	b.ReportAllocs()

	for b.Loop() {
		_ = p.GetKey(ctx, nil)
	}
}
