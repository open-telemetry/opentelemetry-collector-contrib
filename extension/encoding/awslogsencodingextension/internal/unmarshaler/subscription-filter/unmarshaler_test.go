// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestValidateLog(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		log         cloudwatchLogsData
		expectedErr string
	}{
		"valid_cloudwatch_log_data_message": {
			log: cloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogGroup:    "log-group",
				LogStream:   "log-stream",
			},
		},
		"empty_owner_cloudwatch_log_data_message": {
			log: cloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				LogGroup:    "log-group",
				LogStream:   "log-stream",
			},
			expectedErr: errEmptyOwner.Error(),
		},
		"empty_log_group_cloudwatch_log_data_message": {
			log: cloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogStream:   "log-stream",
			},
			expectedErr: errEmptyLogGroup.Error(),
		},
		"empty_log_stream_cloudwatch_log_data_message": {
			log: cloudwatchLogsData{
				MessageType: "DATA_MESSAGE",
				Owner:       "owner",
				LogGroup:    "log-group",
			},
			expectedErr: errEmptyLogStream.Error(),
		},
		"valid_cloudwatch_log_control_message": {
			log: cloudwatchLogsData{
				MessageType: "CONTROL_MESSAGE",
			},
		},
		"invalid_message_type_cloudwatch_log": {
			log: cloudwatchLogsData{
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
func readAndCompressLogFile(t *testing.T, dir, file string) io.Reader {
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
		"valid_cloudwatch_log_control": {
			reader:               readAndCompressLogFile(t, filesDirectory, "valid_cloudwatch_log_control.json"),
			logsExpectedFilename: "valid_cloudwatch_log_control_expected.yaml",
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

// TestUnmarshallCloudwatchLog_MultipleJSONObjects verifies that UnmarshalAWSLogs
// handles multiple concatenated JSON objects in a single reader.
func TestUnmarshallCloudwatchLog_MultipleJSONObjects(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata"
	reader := readAndCompressLogFile(t, filesDirectory, "multiple_json_objects.json")

	unmarshalerCW := NewSubscriptionFilterUnmarshaler(component.BuildInfo{})
	logs, err := unmarshalerCW.UnmarshalAWSLogs(reader)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "multiple_json_objects_expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
}

func TestNewLogsDecoder(t *testing.T) {
	directory := "testdata/stream"
	expectPattern := "subscription_filter_expect_%d.yaml"

	tests := []struct {
		name   string
		offset int64
	}{
		{
			name:   "Normal streaming",
			offset: 0,
		},
		{
			name:   "Streaming with offset",
			offset: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := readAndCompressLogFile(t, directory, "subscription_filter.json")
			unmarshaler := NewSubscriptionFilterUnmarshaler(component.BuildInfo{})

			// flush each record and start with defined offset
			streamer, err := unmarshaler.NewLogsDecoder(content, encoding.WithFlushItems(1), encoding.WithOffset(tt.offset))
			require.NoError(t, err)

			index := tt.offset
			for {
				index++
				var logs plog.Logs
				logs, err = streamer.DecodeLogs()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					t.Errorf("failed to unmarshal log %d: %v", index, err)
				}

				var expectedLogs plog.Logs
				expectedLogs, err = golden.ReadLogs(filepath.Join(directory, fmt.Sprintf(expectPattern, index)))
				require.NoError(t, err)
				require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
				require.Equal(t, index, streamer.Offset())
			}

			// expect EOF after all logs are read
			_, err = streamer.DecodeLogs()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestUnmarshallCloudwatchLog_ExtractedFields(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata"
	tests := map[string]struct {
		reader               io.Reader
		logsExpectedFilename string
	}{
		"extracted_fields_account_id": {
			reader:               readAndCompressLogFile(t, filesDirectory, "extracted_fields_account_id.json"),
			logsExpectedFilename: "extracted_fields_account_id_expected.yaml",
		},
		"extracted_fields_both": {
			reader:               readAndCompressLogFile(t, filesDirectory, "extracted_fields_both.json"),
			logsExpectedFilename: "extracted_fields_both_expected.yaml",
		},
		"two_records_same_extracted_fields": {
			reader:               readAndCompressLogFile(t, filesDirectory, "two_records_same_extracted_fields.json"),
			logsExpectedFilename: "two_records_same_extracted_fields_expected.yaml",
		},
		"two_records_different_extracted_fields": {
			reader:               readAndCompressLogFile(t, filesDirectory, "two_records_different_extracted_fields.json"),
			logsExpectedFilename: "two_records_different_extracted_fields_expected.yaml",
		},
	}

	unmarshalerCW := NewSubscriptionFilterUnmarshaler(component.BuildInfo{})
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logs, err := unmarshalerCW.UnmarshalAWSLogs(test.reader)
			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, test.logsExpectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
		})
	}
}
