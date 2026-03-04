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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
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
func readAndCompressLogFile(t *testing.T, dir, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return compressToGZIPReader(t, data)
}

func readLogFile(t *testing.T, dir, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return bytes.NewReader(data)
}

func TestUnmarshalLogs_PlainText(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := []struct {
		name                 string
		logInputReader       io.Reader
		logsExpectedFilename string
		format               string
		expectedErr          string
		featureGateEnabled   bool
	}{
		{
			name:                 "Valid VPC flow log from S3 - single logs",
			logInputReader:       readAndCompressLogFile(t, dir, "valid_vpc_flow_log_single.log"),
			logsExpectedFilename: "valid_vpc_flow_log_single_expected.yaml",
			featureGateEnabled:   false,
		},
		{
			name:                 "Valid VPC flow log from S3 - multi logs",
			logInputReader:       readAndCompressLogFile(t, dir, "valid_vpc_flow_log_multi.log"),
			logsExpectedFilename: "valid_vpc_flow_log_expected_multi.yaml",
			featureGateEnabled:   false,
		},
		{
			name:                 "Valid VPC flow log from S3 with ISO8601 timestamps",
			logInputReader:       readAndCompressLogFile(t, dir, "valid_vpc_flow_log_multi.log"),
			logsExpectedFilename: "valid_vpc_flow_log_expected_multi_iso8601.yaml",
			featureGateEnabled:   true,
		},
		{
			name:                 "Valid VPC flow log from CloudWatch",
			logInputReader:       readLogFile(t, dir, "valid_vpc_flow_cw.json"),
			logsExpectedFilename: "valid_vpc_flow_cw_expected.yaml",
			featureGateEnabled:   false,
		},
		{
			name:                 "Valid VPC flow log from CloudWatch with customized fields",
			logInputReader:       readLogFile(t, dir, "valid_vpc_flow_cw_custom.json"),
			logsExpectedFilename: "valid_vpc_flow_cw_custom_expected.yaml",
			format:               "version interface-id srcaddr dstaddr",
			featureGateEnabled:   false,
		},
		{
			name:               "Invalid VPC flow log with less fields than required",
			logInputReader:     readAndCompressLogFile(t, dir, "vpc_flow_log_too_few_fields.log"),
			expectedErr:        "log line has less fields than the ones expected",
			featureGateEnabled: false,
		},
		{
			name:               "Invalid VPC flow log with more fields than required",
			logInputReader:     readAndCompressLogFile(t, dir, "vpc_flow_log_too_many_fields.log"),
			expectedErr:        "log line has more fields than the ones expected",
			featureGateEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := Config{
				FileFormat: constants.FileFormatPlainText,
			}

			if test.format != "" {
				config.Format = test.format
			}

			u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), test.featureGateEnabled)
			require.NoError(t, err)

			logs, err := u.UnmarshalAWSLogs(test.logInputReader)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}

			// To generate the golden files, uncomment the following line:
			// golden.WriteLogsToFile(filepath.Join(dir, test.logsExpectedFilename), logs)

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
				"source.address":      "10.40.1.175",
				"destination.address": "10.40.2.236",
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
				"source.address":      "10.40.1.175",
				"destination.address": "10.40.2.236",
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
				"source.address":        "10.20.33.164",
				"destination.address":   "10.40.2.236",
				"network.local.address": "10.40.1.175",
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
				"source.address":        "10.40.2.236",
				"destination.address":   "10.20.33.164",
				"network.local.address": "10.40.2.31",
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
