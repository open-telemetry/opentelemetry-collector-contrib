// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
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
			expectedErr: "invalid first ELB access log line part",
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
		"control message handling": {
			reader:               strings.NewReader("Enable ConnectionLog for ELB\n"),
			logsExpectedFilename: "control_message_expected.yaml",
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

func TestNewLogsDecoder(t *testing.T) {
	directory := "testdata/stream"
	expectPattern := "alb_al_valid_logs_expected_%d.yaml"

	tests := []struct {
		name   string
		offset int64
		index  int
	}{
		{
			name:   "Normal streaming",
			offset: 0,
			index:  0,
		},
		{
			name:   "Stream with offset",
			offset: 577, // skip first record
			index:  1,   // start from first index
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Input with 3 valid ALB logs
			input := readAndCompressLogFile(t, directory, "alb_al_valid_logs.log")

			logger := zaptest.NewLogger(t)
			elbUnmarshaler := NewELBAccessLogUnmarshaler(component.BuildInfo{}, logger)

			// Flush after every log for testing purposes & use set offset from test case
			streamer, err := elbUnmarshaler.NewLogsDecoder(input, encoding.WithFlushItems(1), encoding.WithOffset(tt.offset))
			require.NoError(t, err)

			index := tt.index
			for {
				index++

				var logs plog.Logs
				logs, err = streamer.DecodeLogs()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					t.Errorf("failed to unmarshal log for index %d: %v", index, err)
				}

				// To check or update offset, uncomment offset below
				// fmt.Println(streamer.Offset())

				var expectedLogs plog.Logs
				expectedLogs, err = golden.ReadLogs(filepath.Join(directory, fmt.Sprintf(expectPattern, index)))
				require.NoError(t, err)
				require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
			}

			// expect EOF after all logs are read
			_, err = streamer.DecodeLogs()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func Test_peekAndGetSyntax(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		logSyntax logSyntaxType
		wantError string
	}{
		{
			name:      "Enable message",
			input:     []byte("Enable ConnectionLog for ELB"),
			logSyntax: controlMessage,
			wantError: "",
		},
		{
			name:      "ALB log",
			input:     []byte("http 2018-07-02T22:23:00.186641Z app/my-loadbalancer/50dc6c495c0c9188 192.168.131.39:2817 10.0.0.1:80 0.000 0.001 0.000 200 200 34 366 \"GET http://www.example.com:80/ HTTP/1.1\" \"curl/7.46.0\" - - arn:aws:elasticloadbalancing:us-east-2:123456789012:targetgroup/my-targets/73e2d6bc24d8a067 \"Root=1-58337262-36d228ad5d99923122bbe354\" \"-\" \"-\" 0 2018-07-02T22:22:48.364000Z \"forward\" \"-\" \"-\" \"10.0.0.1:80\" \"200\" \"-\" \"-\" TID_1234abcd5678ef90 \"-\" \"-\" \"-\""),
			logSyntax: albAccessLogs,
			wantError: "",
		},
		{
			name:      "NLB log",
			input:     []byte("tls 2.0 2018-12-20T02:59:40 net/my-network-loadbalancer/c6e77e28c25b2234 g3d4b5e8bb8464cd 72.21.218.154:51341 172.100.100.185:443 5 2 98 246 - arn:aws:acm:us-east-2:671290407336:certificate/2a108f19-aded-46b0-8493-c63eb1ef4a99 - ECDHE-RSA-AES128-SHA tlsv12 - my-network-loadbalancer-c6e77e28c25b2234.elb.us-east-2.amazonaws.com - - - 2018-12-20T02:59:30\n"),
			logSyntax: nlbAccessLogs,
			wantError: "",
		},
		{
			name:      "CLB log",
			input:     []byte("2015-05-13T23:39:43.945958Z my-loadbalancer 192.168.131.39:2817 10.0.0.1:80 0.000073 0.001048 0.000057 200 200 0 29 \"GET http://www.example.com:80/ HTTP/1.1\" \"curl/7.38.0\" - -\n"),
			logSyntax: clbAccessLogs,
			wantError: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(test.input))

			syntax, err := peekAndGetSyntax(reader)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.logSyntax, syntax)

			// reader should not have consumed any bytes
			peekedBytes, _ := reader.ReadBytes('\n')
			require.Equal(t, test.input, peekedBytes)
		})
	}
}
