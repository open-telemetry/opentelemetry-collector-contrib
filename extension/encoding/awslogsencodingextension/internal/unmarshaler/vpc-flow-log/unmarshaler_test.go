// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog

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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
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
		{
			name:                 "Valid Transit Gateway (TGW) flow log",
			logInputReader:       readAndCompressLogFile(t, dir, "valid_tgw_flow_log.log"),
			logsExpectedFilename: "valid_tgw_flow_log_expected.yaml",
			featureGateEnabled:   false,
		},
		{
			name:                 "Valid Transit Gateway (TGW) flow log from CloudWatch",
			logInputReader:       readLogFile(t, dir, "valid_tgw_flow_cw.json"),
			logsExpectedFilename: "valid_tgw_flow_cw_expected.yaml",
			featureGateEnabled:   false,
		},
		{
			name:                 "Valid VPC flow log from S3 with ECS fields",
			logInputReader:       readAndCompressLogFile(t, dir, "valid_vpc_flow_log_ecs.log"),
			logsExpectedFilename: "valid_vpc_flow_log_ecs_expected.yaml",
			featureGateEnabled:   false,
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

func TestNewLogsDecoder(t *testing.T) {
	directory := "testdata/stream"
	expectPattern := "valid_vpc_flow_log_multi_%d.yaml"

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
			offset: 230, // skip first record
			index:  1,   // start from first index
		},
	}

	config := Config{
		FileFormat: constants.FileFormatPlainText,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpcUnmarshal, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
			require.NoError(t, err)

			data, err := os.ReadFile(filepath.Join(directory, "valid_vpc_flow_log_multi.log"))
			require.NoError(t, err)

			// Flush after every log for testing purposes & set offset
			streamer, err := vpcUnmarshal.NewLogsDecoder(bytes.NewReader(data), encoding.WithFlushItems(1), encoding.WithOffset(tt.offset))
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

					t.Errorf("failed to unmarshal log %d: %v", index, err)
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

func TestNewLogsDecoder_offsetForFormats(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid_vpc_flow_cw_custom.json"))
	require.NoError(t, err)

	config := Config{
		FileFormat: constants.FileFormatPlainText,
	}

	vpcUnmarshal, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// Derive decoder with a non-zero offset
	streamer, err := vpcUnmarshal.NewLogsDecoder(bytes.NewReader(data), encoding.WithOffset(50))
	require.NoError(t, err)

	_, err = streamer.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)

	// Byte length of the input
	require.Equal(t, int64(476), streamer.Offset())
}

func TestUnmarshalLogs_Parquet(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	logInputReader := readLogFile(t, dir, "valid_flow_log.parquet")
	logs, err := u.UnmarshalAWSLogs(logInputReader)
	require.NoError(t, err)

	// To generate the golden file, uncomment the following line:
	// golden.WriteLogsToFile(filepath.Join(dir, "valid_flow_log_parquet_expected.yaml"), logs)

	expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "valid_flow_log_parquet_expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
}

func TestNewLogsDecoder_Parquet(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// The test fixture has 4 rows. Use FlushItems=2 to get two batches of 2.
	reader := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder, err := u.NewLogsDecoder(reader, encoding.WithFlushItems(2))
	require.NoError(t, err)

	// First batch: 2 records
	logs1, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 2, logs1.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	// Second batch: 2 records
	logs2, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 2, logs2.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	// Third call: EOF
	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
}

func TestNewLogsDecoder_Parquet_PlainReader(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// Wrap in an io.Reader that does NOT implement io.ReaderAt/io.Seeker,
	// forcing the fallback buffering path.
	data, err := os.ReadFile(filepath.Join("testdata", "valid_flow_log.parquet"))
	require.NoError(t, err)
	reader := io.NopCloser(bytes.NewReader(data)) // NopCloser strips ReaderAt/Seeker

	decoder, err := u.NewLogsDecoder(reader, encoding.WithFlushItems(0))
	require.NoError(t, err)

	logs, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 4, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
}

func TestNewLogsDecoder_Parquet_ReaderAtSeeker(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// *os.File implements io.ReaderAt + io.Seeker but has no Size() method,
	// exercising the readerAtSeeker branch of openParquetFile.
	f, err := os.Open(filepath.Join("testdata", "valid_flow_log.parquet"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	decoder, err := u.NewLogsDecoder(f, encoding.WithFlushItems(0))
	require.NoError(t, err)

	logs, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 4, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
}

func TestNewLogsDecoder_Parquet_SingleBatch(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// Flush disabled (0) returns all rows in one batch.
	reader := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder, err := u.NewLogsDecoder(reader, encoding.WithFlushItems(0))
	require.NoError(t, err)

	logs, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 4, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())

	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
}

func TestNewLogsDecoder_Parquet_Offset(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// Skip first 2 rows, then read the remaining 2.
	reader := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder, err := u.NewLogsDecoder(reader, encoding.WithOffset(2), encoding.WithFlushItems(0))
	require.NoError(t, err)

	require.Equal(t, int64(2), decoder.Offset())

	logs, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 2, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, int64(4), decoder.Offset())

	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
}

func TestNewLogsDecoder_Parquet_OffsetResume(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// First pass: read 2 rows, capture offset.
	reader1 := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder1, err := u.NewLogsDecoder(reader1, encoding.WithFlushItems(2))
	require.NoError(t, err)

	logs1, err := decoder1.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 2, logs1.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	savedOffset := decoder1.Offset()
	require.Equal(t, int64(2), savedOffset)

	// Second pass: resume from saved offset, get remaining rows.
	reader2 := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder2, err := u.NewLogsDecoder(reader2, encoding.WithOffset(savedOffset), encoding.WithFlushItems(0))
	require.NoError(t, err)

	logs2, err := decoder2.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 2, logs2.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, int64(4), decoder2.Offset())
}

func TestNewLogsDecoder_Parquet_OffsetSkipsAll(t *testing.T) {
	t.Parallel()

	config := Config{
		FileFormat: constants.FileFormatParquet,
	}
	u, err := NewVPCFlowLogUnmarshaler(config, component.BuildInfo{}, zap.NewNop(), false)
	require.NoError(t, err)

	// Offset equals total rows — should get EOF immediately.
	reader := readLogFile(t, "testdata", "valid_flow_log.parquet")
	decoder, err := u.NewLogsDecoder(reader, encoding.WithOffset(4), encoding.WithFlushItems(0))
	require.NoError(t, err)

	_, err = decoder.DecodeLogs()
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, int64(4), decoder.Offset())
}

func TestHandleField_ECSFields(t *testing.T) {
	t.Parallel()

	// Each case verifies that the ECS field name maps to the expected attribute key
	// and preserves the input value. Documents the current behavior so it cannot
	// regress back to the pre-existing warn-and-skip path.
	tests := []struct {
		field   string
		value   string
		attrKey string
	}{
		{"ecs-cluster-arn", "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster", "aws.ecs.cluster.arn"},
		{"ecs-cluster-name", "my-cluster", "aws.ecs.cluster.name"},
		{"ecs-container-instance-arn", "arn:aws:ecs:us-east-1:123456789012:container-instance/my-cluster/abcdef", "aws.ecs.container.instance.arn"},
		{"ecs-container-instance-id", "abcdef1234567890", "aws.ecs.container.instance.id"},
		{"ecs-container-id", "container-1", "aws.ecs.container.id"},
		{"ecs-second-container-id", "container-2", "aws.ecs.second.container.id"},
		{"ecs-service-name", "my-service", "aws.ecs.service.name"},
		{"ecs-task-definition-arn", "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1", "aws.ecs.task.definition.arn"},
		{"ecs-task-arn", "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abcdef", "aws.ecs.task.arn"},
		{"ecs-task-id", "abcdef1234567890", "aws.ecs.task.id"},
	}

	v := &VPCFlowLogUnmarshaler{logger: zap.NewNop()}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			record := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			var addr address

			found, err := v.handleField(test.field, test.value, resourceLogs, record, &addr)
			require.NoError(t, err)
			require.True(t, found, "%s should be a recognized field", test.field)

			attr, ok := record.Attributes().Get(test.attrKey)
			require.True(t, ok, "attribute %q not found for field %q", test.attrKey, test.field)
			require.Equal(t, test.value, attr.Str())
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

	v := &VPCFlowLogUnmarshaler{logger: zap.NewNop()}
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
