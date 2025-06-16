// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// compressToGZIPReader compresses buf into a gzip-formatted io.Reader.
func compressToGZIPReader(t *testing.T, buf []byte) io.Reader {
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err := gzipWriter.Write(buf)
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)
	gzipReader, err := gzip.NewReader(bytes.NewReader(compressedData.Bytes()))
	require.NoError(t, err)
	return gzipReader
}

// readAndCompressLogFile reads the data inside it, compresses it
// and returns a GZIP reader for it.
func readAndCompressLogFile(t *testing.T, dir string, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return compressToGZIPReader(t, data)
}

func TestUnmarshalLogs_PlainText(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		reader               io.Reader
		logsExpectedFilename string
		expectedErr          string
	}{
		"valid_vpc_flow_log": {
			reader:               readAndCompressLogFile(t, dir, "valid_vpc_flow_log.log"),
			logsExpectedFilename: "valid_vpc_flow_log_expected.yaml",
		},
		"vpc_flow_log_with_more_fields_than_allowed": {
			reader:      readAndCompressLogFile(t, dir, "vpc_flow_log_too_few_fields.log"),
			expectedErr: "log line has less fields than the ones expected",
		},
		"vpc_flow_log_with_less_fields_than_required": {
			reader:      readAndCompressLogFile(t, dir, "vpc_flow_log_too_many_fields.log"),
			expectedErr: "log line has more fields than the ones expected",
		},
	}

	u, err := NewVPCFlowLogUnmarshaler(fileFormatPlainText, component.BuildInfo{}, zap.NewNop())
	require.NoError(t, err)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := u.UnmarshalAWSLogs(test.reader)

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

	// We will test the address using the examples at
	// https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html#flow-log-example-nat
	tests := map[string]struct {
		original address
		expected map[string]string
	}{
		"no_pkt": {
			original: address{
				source:      "10.40.1.175",
				destination: "10.40.2.236",
			},
			expected: map[string]string{
				string(conventions.SourceAddressKey):      "10.40.1.175",
				string(conventions.DestinationAddressKey): "10.40.2.236",
			},
		},
		"pkt_same_as_src-dst": {
			original: address{
				source:         "10.40.1.175",
				destination:    "10.40.2.236",
				pktSource:      "10.40.1.175",
				pktDestination: "10.40.2.236",
			},
			expected: map[string]string{
				string(conventions.SourceAddressKey):      "10.40.1.175",
				string(conventions.DestinationAddressKey): "10.40.2.236",
			},
		},
		"different_source": {
			original: address{
				source:         "10.40.1.175",
				destination:    "10.40.2.236",
				pktSource:      "10.20.33.164",
				pktDestination: "10.40.2.236",
			},
			expected: map[string]string{
				string(conventions.SourceAddressKey):       "10.20.33.164",
				string(conventions.DestinationAddressKey):  "10.40.2.236",
				string(conventions.NetworkLocalAddressKey): "10.40.1.175",
			},
		},
		"different_destination": {
			original: address{
				source:         "10.40.2.236",
				destination:    "10.40.2.31",
				pktSource:      "10.40.2.236",
				pktDestination: "10.20.33.164",
			},
			expected: map[string]string{
				string(conventions.SourceAddressKey):       "10.40.2.236",
				string(conventions.DestinationAddressKey):  "10.20.33.164",
				string(conventions.NetworkLocalAddressKey): "10.40.2.31",
			},
		},
	}

	v := &vpcFlowLogUnmarshaler{logger: zap.NewNop()}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			v.handleAddresses(&test.original, record)
			for attr, value := range test.expected {
				addr, found := record.Attributes().Get(attr)
				require.True(t, found)
				require.Equal(t, value, addr.Str())
			}
		})
	}
}
