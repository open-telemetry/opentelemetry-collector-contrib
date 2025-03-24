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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestVPCFlowLog(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field       string
		value       string
		expectedErr string
	}{
		"unknown_value": {
			field: "test",
			value: "-",
		},
		"unknown_field": {
			field: "test",
			value: "test",
		},
		"supported_value": {
			field: "action",
			value: "ACCEPT",
		},
		"unsupported_value": {
			field:       "action",
			value:       "unsupported",
			expectedErr: `value "unsupported" is invalid for the action field, valid values are [ACCEPT REJECT]`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateVPCFlowLog(test.field, test.value)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// compressData in gzip format
func compressData(t *testing.T, buf []byte) []byte {
	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err := gzipWriter.Write(buf)
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)
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

	u, errCreate := NewVPCFlowLogUnmarshaler(fileFormatPlainText, component.BuildInfo{})
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
