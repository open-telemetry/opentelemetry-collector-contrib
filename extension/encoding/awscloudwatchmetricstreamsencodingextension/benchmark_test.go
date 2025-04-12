// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"bytes"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func createJSONFileContent(b *testing.B, filename string, nLogs int) []byte {
	data, err := os.ReadFile(filename)
	require.NoError(b, err)

	// remove all insignificant spaces,
	// including new lines
	var compacted bytes.Buffer
	err = gojson.Compact(&compacted, data)
	require.NoError(b, err)

	result := make([][]byte, nLogs)
	for i := 0; i < nLogs; i++ {
		result[i] = compacted.Bytes()
	}
	return bytes.Join(result, []byte{'\n'})
}

func BenchmarkUnmarshalJSONMetrics(b *testing.B) {
	// each log line of this file is around 80B
	filename := "./testdata/json/valid_metric.json"

	tests := map[string]struct {
		nLogs int
	}{
		"1_metrics": {
			nLogs: 1,
		},
		"1000_metrics": {
			nLogs: 1_000,
		},
	}

	u := formatJSONUnmarshaler{
		buildInfo: component.BuildInfo{},
	}

	for name, test := range tests {
		data := createJSONFileContent(b, filename, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := u.UnmarshalMetrics(data)
				require.NoError(b, err)
			}
		})
	}
}
