// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"bytes"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// newWAFLogContent reads the testdata/valid_log.json file, and creates
// a byte array with nLogs line, in which each line is the compacted
// log.
func newWAFLogContent(b *testing.B, nLogs int) []byte {
	data, err := os.ReadFile("testdata/valid_log.json")
	require.NoError(b, err)

	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(b, err)

	compactedBytes := compacted.Bytes()
	result := make([][]byte, nLogs)
	for i := 0; i < nLogs; i++ {
		result[i] = compactedBytes
	}
	return bytes.Join(result, []byte{'\n'})
}

func BenchmarkUnmarshalLogs(b *testing.B) {
	tests := map[string]struct {
		nLogs int
	}{
		"1_log": {
			nLogs: 1,
		},
		"1000_logs": {
			nLogs: 1_000,
		},
	}

	u := wafLogUnmarshaler{
		buildInfo: component.BuildInfo{},
	}

	for name, test := range tests {
		data := newWAFLogContent(b, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := u.UnmarshalAWSLogs(bytes.NewReader(data))
				require.NoError(b, err)
			}
		})
	}
}
