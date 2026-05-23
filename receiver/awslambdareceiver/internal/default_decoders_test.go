// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestDefaultS3LogsDecoder_NewLogsDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        func() io.Reader
		expectedPath string
	}{
		{
			name: "plain text input",
			input: func() io.Reader {
				return bytes.NewReader([]byte("some text"))
			},
			expectedPath: filepath.Join("testdata", "default_s3_decoder_expected.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decoder := NewDefaultS3LogsDecoder()
			logsDecoder, err := decoder.NewLogsDecoder(tt.input())
			require.NoError(t, err)
			require.NotNil(t, logsDecoder)

			// First call should return logs
			logs, err := logsDecoder.DecodeLogs()
			require.NoError(t, err)
			require.Equal(t, 1, logs.ResourceLogs().Len())

			// Validate against expected golden file
			// Uncomment below to update the expected files
			// golden.WriteLogs(t, tt.expectedPath, logs)
			expectedLogs, err := golden.ReadLogs(tt.expectedPath)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))

			// Second call should return EOF
			_, err = logsDecoder.DecodeLogs()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestDefaultCWLogsDecoder_NewLogsDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        func() io.Reader
		expectedPath string
	}{
		{
			name: "plain text CloudWatch logs JSON",
			input: func() io.Reader {
				return bytes.NewReader([]byte(`{"owner":"111222333444","logGroup":"/test/log-group","logStream":"test-stream","messageType":"DATA_MESSAGE","logEvents":[{"id":"1","timestamp":1700000000000,"message":"some text"}]}`))
			},
			expectedPath: filepath.Join("testdata", "default_cw_decoder_expected.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decoder := NewDefaultCWLogsDecoder()
			logsDecoder, err := decoder.NewLogsDecoder(tt.input())
			require.NoError(t, err)
			require.NotNil(t, logsDecoder)

			// First call should return logs
			logs, err := logsDecoder.DecodeLogs()
			require.NoError(t, err)
			require.Equal(t, 1, logs.ResourceLogs().Len())

			// Validate against expected golden file
			// Uncomment below to update the expected files
			// golden.WriteLogs(t, tt.expectedPath, logs)
			expectedLogs, err := golden.ReadLogs(tt.expectedPath)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))

			// Second call should return EOF
			_, err = logsDecoder.DecodeLogs()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}
