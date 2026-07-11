// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpcflowlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
)

func createVPCFlowLogContent(b *testing.B, filename string, nLogs int) []byte {
	data, err := os.ReadFile(filename)
	require.NoError(b, err)
	lines := bytes.Split(data, []byte{'\n'})
	if len(lines) < 2 {
		require.Fail(b, "file should have at least 1 line for fields and 1 line for the VPC flow log")
	}

	fieldLine := lines[0]
	flowLog := lines[1]

	result := make([][]byte, nLogs+1)
	result[0] = fieldLine
	for i := range nLogs {
		result[i+1] = flowLog
	}
	return bytes.Join(result, []byte{'\n'})
}

// vpcFlowLogRow mirrors the Parquet schema used by VPC Flow Logs.
type vpcFlowLogRow struct {
	Version     int32  `parquet:"version"`
	AccountID   string `parquet:"account_id"`
	InterfaceID string `parquet:"interface_id"`
	Srcaddr     string `parquet:"srcaddr"`
	Dstaddr     string `parquet:"dstaddr"`
	Srcport     int32  `parquet:"srcport"`
	Dstport     int32  `parquet:"dstport"`
	Protocol    int32  `parquet:"protocol"`
	Packets     int64  `parquet:"packets"`
	Bytes       int64  `parquet:"bytes"`
	Start       int64  `parquet:"start"`
	End         int64  `parquet:"end"`
	Action      string `parquet:"action"`
	LogStatus   string `parquet:"log_status"`
	VpcID       string `parquet:"vpc_id"`
	SubnetID    string `parquet:"subnet_id"`
	InstanceID  string `parquet:"instance_id"`
	TCPFlags    int32  `parquet:"tcp_flags"`
	AzID        string `parquet:"az_id"`
	Type        string `parquet:"type"`
	PktSrcaddr  string `parquet:"pkt_srcaddr"`
	PktDstaddr  string `parquet:"pkt_dstaddr"`
	Region      string `parquet:"region"`
	TrafficPath int32  `parquet:"traffic_path"`
}

// rowsPerRowGroup controls how many rows go into each Parquet row group.
const rowsPerRowGroup = 10_000

func createVPCFlowLogParquetContent(b *testing.B, nLogs int) []byte {
	b.Helper()
	row := vpcFlowLogRow{
		Version:     5,
		AccountID:   "123456789012",
		InterfaceID: "eni-0abc1234def567890",
		Srcaddr:     "10.0.1.100",
		Dstaddr:     "10.0.2.200",
		Srcport:     49152,
		Dstport:     22,
		Protocol:    6,
		Packets:     15,
		Bytes:       3500,
		Start:       1700000000,
		End:         1700000060,
		Action:      "ACCEPT",
		LogStatus:   "OK",
		VpcID:       "vpc-0aaa1111bbbb2222c",
		SubnetID:    "subnet-0ddd3333eeee4444f",
		InstanceID:  "i-0abcdef1234567890",
		TCPFlags:    2,
		AzID:        "use1-az1",
		Type:        "IPv4",
		PktSrcaddr:  "203.0.113.50",
		PktDstaddr:  "10.0.2.200",
		Region:      "us-east-1",
		TrafficPath: 1,
	}

	schema := parquet.SchemaOf(new(vpcFlowLogRow))
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[vpcFlowLogRow](&buf, schema)

	for written := 0; written < nLogs; {
		groupSize := rowsPerRowGroup
		if written+groupSize > nLogs {
			groupSize = nLogs - written
		}
		rows := make([]vpcFlowLogRow, groupSize)
		for i := range rows {
			rows[i] = row
		}
		_, err := w.Write(rows)
		require.NoError(b, err)
		require.NoError(b, w.Flush())
		written += groupSize
	}
	require.NoError(b, w.Close())
	return buf.Bytes()
}

func BenchmarkUnmarshalParquetLogs(b *testing.B) {
	u := VPCFlowLogUnmarshaler{
		cfg:       Config{FileFormat: constants.FileFormatParquet},
		buildInfo: component.BuildInfo{},
		logger:    zap.NewNop(),
	}

	data := createVPCFlowLogParquetContent(b, 50_000)

	b.ReportAllocs()
	for b.Loop() {
		_, err := u.UnmarshalAWSLogs(bytes.NewReader(data))
		require.NoError(b, err)
	}
}

func BenchmarkUnmarshalPlainTextLogs(b *testing.B) {
	u := VPCFlowLogUnmarshaler{
		cfg:       Config{FileFormat: constants.FileFormatPlainText},
		buildInfo: component.BuildInfo{},
		logger:    zap.NewNop(),
	}

	data := createVPCFlowLogContent(b, "./testdata/valid_vpc_flow_log_single.log", 50_000)

	b.ReportAllocs()
	for b.Loop() {
		_, err := u.UnmarshalAWSLogs(bytes.NewReader(data))
		require.NoError(b, err)
	}
}

func BenchmarkStreamDecodePlainTextLogs(b *testing.B) {
	filename := "./testdata/valid_vpc_flow_log_single.log"

	tests := map[string]struct {
		nLogs      int
		flushItems int64
	}{
		"50000_logs/default_flush": {
			nLogs:      50_000,
			flushItems: 1_000,
		},
		"50000_logs/flush_100": {
			nLogs:      50_000,
			flushItems: 100,
		},
	}

	u := VPCFlowLogUnmarshaler{
		cfg:       Config{FileFormat: constants.FileFormatPlainText},
		buildInfo: component.BuildInfo{},
		logger:    zap.NewNop(),
	}

	for name, test := range tests {
		data := createVPCFlowLogContent(b, filename, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				decoder, err := u.NewLogsDecoder(bytes.NewReader(data), encoding.WithFlushItems(test.flushItems))
				require.NoError(b, err)
				for {
					_, err = decoder.DecodeLogs()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(b, err)
				}
			}
		})
	}
}

func BenchmarkStreamDecodeParquetLogs(b *testing.B) {
	tests := map[string]struct {
		nLogs      int
		flushItems int64
	}{
		"50000_logs/default_flush": {
			nLogs:      50_000,
			flushItems: 1_000,
		},
		"50000_logs/flush_100": {
			nLogs:      50_000,
			flushItems: 100,
		},
	}

	u := VPCFlowLogUnmarshaler{
		cfg:       Config{FileFormat: constants.FileFormatParquet},
		buildInfo: component.BuildInfo{},
		logger:    zap.NewNop(),
	}

	for name, test := range tests {
		data := createVPCFlowLogParquetContent(b, test.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				decoder, err := u.NewLogsDecoder(bytes.NewReader(data), encoding.WithFlushItems(test.flushItems))
				require.NoError(b, err)
				for {
					_, err = decoder.DecodeLogs()
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(b, err)
				}
			}
		})
	}
}
