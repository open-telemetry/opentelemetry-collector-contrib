// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// compressData in gzip format
func compressData(tb testing.TB, buf []byte) []byte {
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err := gzipWriter.Write(buf)
	require.NoError(tb, err)
	err = gzipWriter.Close()
	require.NoError(tb, err)
	return compressedData.Bytes()
}

// getLogFromFileInPlainText reads the file content
// returns it in the format the data is expected to
// be: in plain text and gzip compressed.
func getLogFromFileInPlainText(t *testing.T, dir string, file string) []byte {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return compressData(t, data)
}

func TestUnmarshalLogs_PlainText(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		content              []byte
		logsExpectedFilename string
		expectedErr          string
	}{
		"valid_vpc_flow_log": {
			content:              getLogFromFileInPlainText(t, dir, "valid_vpc_flow_log.log"),
			logsExpectedFilename: "valid_vpc_flow_log_expected.yaml",
		},
		"invalid_vpc_flow_log": {
			content:     getLogFromFileInPlainText(t, dir, "invalid_vpc_flow_log.log"),
			expectedErr: "expect 14 fields per log line, got log line with 13 fields",
		},
		"invalid_gzip_record": {
			content:     []byte("invalid"),
			expectedErr: "failed to decompress content",
		},
	}

	u, errCreate := NewVPCFlowLogUnmarshaler(fileFormatPlainText, component.BuildInfo{}, zap.NewNop())
	require.NoError(t, errCreate)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := u.UnmarshalLogs(test.content)

			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.logsExpectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}

func TestHandleAddresses(t *testing.T) {
	t.Parallel()

	// We will test the addresses using the examples at
	// https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat
	tests := map[string]struct {
		original map[string]string
		expected map[string]string
	}{
		"no_pkt": {
			original: map[string]string{
				"srcaddr": "10.40.1.175",
				"dstaddr": "10.40.2.236",
			},
			expected: map[string]string{
				conventions.AttributeSourceAddress:      "10.40.1.175",
				conventions.AttributeDestinationAddress: "10.40.2.236",
			},
		},
		"pkt_same_as_src-dst": {
			original: map[string]string{
				"srcaddr":     "10.40.1.175",
				"dstaddr":     "10.40.2.236",
				"pkt-srcaddr": "10.40.1.175",
				"pkt-dstaddr": "10.40.2.236",
			},
			expected: map[string]string{
				conventions.AttributeSourceAddress:      "10.40.1.175",
				conventions.AttributeDestinationAddress: "10.40.2.236",
			},
		},
		"different_source": {
			original: map[string]string{
				"srcaddr":     "10.40.1.175",
				"dstaddr":     "10.40.2.236",
				"pkt-srcaddr": "10.20.33.164",
				"pkt-dstaddr": "10.40.2.236",
			},
			expected: map[string]string{
				conventions.AttributeSourceAddress:      "10.20.33.164",
				conventions.AttributeDestinationAddress: "10.40.2.236",
				conventions.AttributeNetworkPeerAddress: "10.40.1.175",
			},
		},
		"different_destination": {
			original: map[string]string{
				"srcaddr":     "10.40.2.236",
				"dstaddr":     "10.40.2.31",
				"pkt-srcaddr": "10.40.2.236",
				"pkt-dstaddr": "10.20.33.164",
			},
			expected: map[string]string{
				conventions.AttributeSourceAddress:      "10.40.2.236",
				conventions.AttributeDestinationAddress: "10.20.33.164",
				conventions.AttributeNetworkPeerAddress: "10.40.2.31",
			},
		},
	}

	v := &vpcFlowLogUnmarshaler{logger: zap.NewNop()}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := v.handleAddresses(test.original)
			require.Equal(t, test.expected, result)
		})
	}
}

func TestUnmarshalLogs_Parquet(t *testing.T) {
	_, errCreate := NewVPCFlowLogUnmarshaler(fileFormatParquet, component.BuildInfo{}, zap.NewNop())
	require.ErrorContains(t, errCreate, "still needs to be implemented")
}

func TestUnmarshalLogs_Unsupported(t *testing.T) {
	_, errCreate := NewVPCFlowLogUnmarshaler("unsupported", component.BuildInfo{}, zap.NewNop())
	require.ErrorContains(t, errCreate, `unsupported file fileFormat "unsupported" for VPC flow log`)
}
