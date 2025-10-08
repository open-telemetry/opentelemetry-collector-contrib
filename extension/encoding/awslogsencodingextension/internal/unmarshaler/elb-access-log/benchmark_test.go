// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type elbBenchmarkCase struct {
	name     string
	filename string
	nLogs    int
}

var elbCases = []elbBenchmarkCase{
	{"ALB_1_log", "./testdata/alb_al_valid_logs.log", 1},
	{"ALB_1000_logs", "./testdata/alb_al_valid_logs.log", 1_000},
	{"CLB_1_log", "./testdata/clb_al_valid_logs.log", 1},
	{"CLB_1000_logs", "./testdata/clb_al_valid_logs.log", 1_000},
	{"NLB_1_log", "./testdata/nlb_al_valid_logs.log", 1},
	{"NLB_1000_logs", "./testdata/nlb_al_valid_logs.log", 1_000},
}

// createELBAccessLogContent reads data from a given elb log file
// to get a valid log line. It will add this log line to a byte array
// and append this line as many times as it takes to reach the desire
// number of logs defined in nLogs. Each log record is defined in one
// line. A new line means a new log record.
func createELBAccessLogContent(b *testing.B, filename string, nLogs int) []byte {
	// get a log line from the testdata
	data, err := os.ReadFile(filename)
	require.NoError(b, err)

	size := len(data) + 1 // + "\n"
	buf := bytes.NewBuffer(make([]byte, 0, nLogs*size))
	for i := range nLogs {
		buf.Write(data)
		if i != nLogs-1 {
			buf.WriteString("\n")
		}
	}

	return buf.Bytes()
}

func BenchmarkUnmarshalAWSLogs(b *testing.B) {
	u := &elbAccessLogUnmarshaler{
		buildInfo: component.BuildInfo{},
		logger:    zap.NewNop(),
	}
	for _, bc := range elbCases {
		data := createELBAccessLogContent(b, bc.filename, bc.nLogs)
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := u.UnmarshalAWSLogs(bytes.NewReader(data))
				require.NoError(b, err)
			}
		})
	}
}
