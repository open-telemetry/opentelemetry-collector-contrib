// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
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
	for i := range nLogs {
		result[i+1] = flowLog
	}
	return bytes.Join(result, []byte{'\n'})
}

func BenchmarkUnmarshalUnmarshalPlainTextLogs(b *testing.B) {
	// each log line of this file is around 80B
	filename := "./testdata/valid_vpc_flow_log_single.log"

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

	factory, err := NewVPCFlowLogUnmarshalerFactory(
		Config{FileFormat: constants.FileFormatPlainText},
		component.BuildInfo{},
		zap.NewNop(),
		false,
	)
	require.NoError(b, err)

	for name, test := range tests {
		data := createVPCFlowLogContent(b, filename, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				decoder, err := factory(bytes.NewReader(data), encoding.StreamDecoderOptions{})
				require.NoError(b, err)
				logs := plog.NewLogs()
				err = decoder.Decode(context.Background(), logs)
				require.NoError(b, err)
			}
		})
	}
}
