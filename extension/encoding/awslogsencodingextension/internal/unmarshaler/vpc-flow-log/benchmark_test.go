// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bytes"
	"os"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func createVPCFlowLogContent(b *testing.B, filename string, nLogs int) []byte {
	data, err := os.ReadFile(filename)
	require.NoError(b, err)
	lines := bytes.Split(data, []byte{'\n'})
	if len(lines) < 2 {
		require.Fail(b, "file should have at least 1 line for fields and 1 line for the VPC flow log")
	}

	fieldLine := lines[0]
	flowLog := lines[1]

	result := make([][]byte, nLogs+1)
	result[0] = fieldLine
	for i := 0; i < nLogs; i++ {
		result[i+1] = flowLog
	}
	final := bytes.Join(result, []byte{'\n'})

	return compressData(b, final)
}

func BenchmarkUnmarshalUnmarshalPlainTextLogs(b *testing.B) {
	// each log line of this file is around 80B
	filename := "./testdata/valid_vpc_flow_log.log"

	tests := map[string]struct {
		nLogs int
	}{
		"small_file": {
			nLogs: 1_000, // 80K
		},
		"medium_file": {
			nLogs: 100_000, // 8MB
		},
		"large_file": {
			nLogs: 1_000_000, // 80MB
		},
	}

	u := vpcFlowLogUnmarshaler{
		fileFormat: fileFormatPlainText,
		buildInfo:  component.BuildInfo{},
		logger:     zap.NewNop(),
	}

	for name, test := range tests {
		data := createVPCFlowLogContent(b, filename, test.nLogs)
		gzipReader, errGzipReader := gzip.NewReader(bytes.NewReader(data))
		require.NoError(b, errGzipReader)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := u.unmarshalPlainTextLogs(gzipReader)
				require.NoError(b, err)
			}
		})
	}
}
