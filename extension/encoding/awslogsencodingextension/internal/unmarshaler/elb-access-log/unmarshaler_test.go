// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap/zaptest"

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
// in case of file containing uncompressed clb logs it returns a bytes.NewReader.
func readAndCompressLogFile(t *testing.T, dir, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	// CLB logs do not arrive in gzip format but in plain text
	if strings.Contains(file, "clb_al") {
		return bytes.NewReader(data)
	}
	return compressToGZIPReader(t, data)
}

func TestUnmarshallELBAccessLogs(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata"
	tests := map[string]struct {
		reader               io.Reader
		logsExpectedFilename string
		expectedErr          string
	}{
		"valid alb log": {
			reader:               readAndCompressLogFile(t, filesDirectory, "alb_al_valid_logs.log"),
			logsExpectedFilename: "alb_al_valid_logs_expected.yaml",
		},
		"empty log line": {
			reader:      readAndCompressLogFile(t, filesDirectory, "elb_empty_line.log"),
			expectedErr: "log line has no fields:",
		},
		"empty log file": {
			reader:      readAndCompressLogFile(t, filesDirectory, "elb_empty_file.log"),
			expectedErr: "no log lines found",
		},
		"invalid syntax field": {
			reader:      readAndCompressLogFile(t, filesDirectory, "alb_al_invalid_syntax.log"),
			expectedErr: "unable to determine log syntax: invalid type: invalid",
		},
		"insufficient alb fields": {
			reader:      readAndCompressLogFile(t, filesDirectory, "alb_al_insufficient_fields.log"),
			expectedErr: "alb access logs do not have enough fields. Expected 29, got 3",
		},
		"valid nlb log": {
			reader:               readAndCompressLogFile(t, filesDirectory, "nlb_al_valid_logs.log"),
			logsExpectedFilename: "nlb_al_valid_logs_expected.yaml",
		},
		"insufficient nlb fields": {
			reader:      readAndCompressLogFile(t, filesDirectory, "nlb_al_insufficient_fields.log"),
			expectedErr: "nlb access logs do not have enough fields. Expected 22, got 3",
		},
		"valid clb log": {
			reader:               readAndCompressLogFile(t, filesDirectory, "clb_al_valid_logs.log"),
			logsExpectedFilename: "clb_al_valid_logs_expected.yaml",
		},
		"insufficient CLB fields": {
			reader:      readAndCompressLogFile(t, filesDirectory, "clb_al_insufficient_fields.log"),
			expectedErr: "clb access logs do not have enough fields. Expected 15, got 3",
		},
		"process elb control file": {
			reader: readAndCompressLogFile(t, filesDirectory, "elb_control.log"),
		},
		"invalid syntax": {
			reader:      readAndCompressLogFile(t, filesDirectory, "elb_invalid_syntax.log"),
			expectedErr: "failed to parse log line",
		},
	}
	// Create a mock logger
	logger := zaptest.NewLogger(t)
	elbUnmarshaler := NewELBAccessLogUnmarshaler(component.BuildInfo{}, logger)
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := elbUnmarshaler.UnmarshalAWSLogs(test.reader)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			// uncomment this to generate the expected file
			// golden.WriteLogs(t, filepath.Join(filesDirectory, test.logsExpectedFilename), logs)

			if test.logsExpectedFilename == "" {
				return
			}
			expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, test.logsExpectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
		})
	}
}
