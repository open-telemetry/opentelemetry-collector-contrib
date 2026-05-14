// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkUnmarshal compares unmarshalEventXML (full) against unmarshalRawEventXML
// (minimal) across payloads of different complexity. The raw path is used when
// raw=true and skips populating fields that sendEvent does not read in that mode.
func BenchmarkUnmarshal(b *testing.B) {
	cases := []struct {
		name string
		file string
	}{
		// Simple event: no RenderingInfo, small EventData payload.
		{"simple", "xmlSample.xml"},
		// Complex event: large RenderingInfo section with message text, keywords,
		// provider strings etc. — the section raw unmarshal skips entirely.
		{"withRenderingInfo", "xmlRenderingInfoSecurity.xml"},
	}

	for _, tc := range cases {
		data, err := os.ReadFile(filepath.Join("testdata", tc.file))
		require.NoError(b, err)

		b.Run(tc.name+"/full", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = unmarshalEventXML(data)
			}
		})

		b.Run(tc.name+"/raw", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = unmarshalRawEventXML(data)
			}
		})
	}
}
