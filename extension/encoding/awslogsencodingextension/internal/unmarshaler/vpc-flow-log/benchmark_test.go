// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bytes"
	"os"
	"testing"

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
	return bytes.Join(result, []byte{'\n'})
}

func BenchmarkUnmarshalUnmarshalPlainTextLogs(b *testing.B) {
	// each log line of this file is around 80B
	filename := "./testdata/valid_vpc_flow_log.log"

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

	u := vpcFlowLogUnmarshaler{
		fileFormat: fileFormatPlainText,
		buildInfo:  component.BuildInfo{},
		logger:     zap.NewNop(),
	}

	for name, test := range tests {
		data := createVPCFlowLogContent(b, filename, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := u.unmarshalPlainTextLogs(bytes.NewReader(data))
				require.NoError(b, err)
			}
		})
	}
}
