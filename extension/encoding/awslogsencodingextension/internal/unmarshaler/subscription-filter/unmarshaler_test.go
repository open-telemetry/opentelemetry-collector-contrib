// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestValidateLog(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		log         events.CloudwatchLogsData
		expectedErr string
	}{
		"valid_cloudwatch_log_data_message": {
			log: events.CloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogGroup:    "log-group",
				LogStream:   "log-stream",
			},
		},
		"empty_owner_cloudwatch_log_data_message": {
			log: events.CloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				LogGroup:    "log-group",
				LogStream:   "log-stream",
			},
			expectedErr: errEmptyOwner.Error(),
		},
		"empty_log_group_cloudwatch_log_data_message": {
			log: events.CloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogStream:   "log-stream",
			},
			expectedErr: errEmptyLogGroup.Error(),
		},
		"empty_log_stream_cloudwatch_log_data_message": {
			log: events.CloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogGroup:    "log-group",
			},
			expectedErr: errEmptyLogStream.Error(),
		},
		"valid_cloudwatch_log_control_message": {
			log: events.CloudwatchLogsData{
				MessageType: "CONTROL_MESSAGE",
			},
		},
		"invalid_message_type_cloudwatch_log": {
			log: events.CloudwatchLogsData{
				MessageType: "INVALID",
			},
			expectedErr: `cloudwatch log has invalid message type "INVALID"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateLog(test.log)
			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.expectedErr)
			}
		})
	}
}

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

func TestUnmarshallCloudwatchLog_SubscriptionFilter(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata"
	tests := map[string]struct {
		reader               io.Reader
		logsExpectedFilename string
		expectedErr          string
	}{
		"valid_cloudwatch_log": {
			reader:               readAndCompressLogFile(t, filesDirectory, "valid_cloudwatch_log.json"),
			logsExpectedFilename: "valid_cloudwatch_log_expected.yaml",
		},
		"invalid_cloudwatch_log": {
			reader:      readAndCompressLogFile(t, filesDirectory, "invalid_cloudwatch_log.json"),
			expectedErr: "invalid cloudwatch log",
		},
		"invalid_gzip_record": {
			reader:      bytes.NewReader([]byte("invalid")),
			expectedErr: "failed to decode decompressed reader",
		},
		"invalid_json_struct": {
			reader:      compressToGZIPReader(t, []byte("invalid")),
			expectedErr: "failed to decode decompressed reader",
		},
	}

	unmarshalerCW := NewSubscriptionFilterUnmarshaler(component.BuildInfo{})
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := unmarshalerCW.UnmarshalAWSLogs(test.reader)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)
			expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, test.logsExpectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}
