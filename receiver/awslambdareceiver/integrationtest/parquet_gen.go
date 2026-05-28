// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/integrationtest"

import (
	"bytes"
	"fmt"
	"time"

	"github.com/parquet-go/parquet-go"
)

// vpcFlowLogRow mirrors the Parquet schema used by awslogsencodingextension for VPC flow logs.
// Field names use underscores; the decoder converts them to hyphen-separated attribute names.
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

const rowsPerRowGroup = 10_000

// generateVPCFlowParquet returns a Parquet-encoded byte slice containing numRows synthetic VPC
// flow log records. Source IPs and ports vary per row to prevent high compression ratios and
// ensure the output is several MB for numRows ≥ 100 000, which spans more than one 4 MB chunk
// in the receiver's range-GET reader.
func generateVPCFlowParquet(numRows int) []byte {
	schema := parquet.SchemaOf(new(vpcFlowLogRow))
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[vpcFlowLogRow](&buf, schema)

	now := time.Now().Unix()
	batch := make([]vpcFlowLogRow, rowsPerRowGroup)

	for written := 0; written < numRows; {
		n := rowsPerRowGroup
		if written+n > numRows {
			n = numRows - written
		}
		for i := range n {
			idx := written + i
			batch[i] = vpcFlowLogRow{
				Version:     2,
				AccountID:   "123456789012",
				InterfaceID: fmt.Sprintf("eni-%08x", idx),
				Srcaddr:     fmt.Sprintf("10.%d.%d.%d", (idx>>16)&0xff, (idx>>8)&0xff, idx&0xff),
				Dstaddr:     fmt.Sprintf("192.168.%d.%d", (idx>>8)&0xff, idx&0xff),
				Srcport:     int32(1024 + idx%64511),
				Dstport:     443,
				Protocol:    6,
				Packets:     int64(1 + idx%100),
				Bytes:       int64(64 + idx%65472),
				Start:       now + int64(idx),
				End:         now + int64(idx) + 60,
				Action:      "ACCEPT",
				LogStatus:   "OK",
				VpcID:       "vpc-0123456789abcdef0",
				SubnetID:    fmt.Sprintf("subnet-%08x", idx/256),
				InstanceID:  fmt.Sprintf("i-%08x", idx/16),
				TCPFlags:    2,
				AzID:        "use1-az1",
				Type:        "IPv4",
				PktSrcaddr:  fmt.Sprintf("10.%d.%d.%d", (idx>>16)&0xff, (idx>>8)&0xff, idx&0xff),
				PktDstaddr:  fmt.Sprintf("192.168.%d.%d", (idx>>8)&0xff, idx&0xff),
				Region:      "us-east-1",
				TrafficPath: 1,
			}
		}
		if _, err := w.Write(batch[:n]); err != nil {
			panic(fmt.Sprintf("parquet write: %v", err))
		}
		if err := w.Flush(); err != nil {
			panic(fmt.Sprintf("parquet flush: %v", err))
		}
		written += n
	}

	if err := w.Close(); err != nil {
		panic(fmt.Sprintf("parquet close: %v", err))
	}

	return buf.Bytes()
}
