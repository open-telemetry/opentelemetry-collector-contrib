// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// newLogFileContent reads data from testdata/valid_s3_access_log.log
// to get a valid log line. It will add this log line to a byte array
// and append this line as many times as it takes to reach the desire
// number of logs defined in nLogs. Each log record is defined in one
// line. A new line means a new log record.
func newLogFileContent(b *testing.B, nLogs int) []byte {
	// get a log line from the testdata
	data, err := os.ReadFile("testdata/valid_s3_access_log.log")
	require.NoError(b, err)

	size := len(data) + 1 // + "\n"
	buf := bytes.NewBuffer(make([]byte, 0, nLogs*size))
	for i := 0; i < nLogs; i++ {
		buf.Write(data)
		if i != nLogs-1 {
			buf.Write([]byte("\n"))
		}
	}

	return buf.Bytes()
}

func BenchmarkUnmarshalLogs(b *testing.B) {
	benchmarks := map[string]struct {
		nLogs int
	}{
		"1_log": {
			nLogs: 1,
		},
		"1000_logs": {
			nLogs: 1_000,
		},
	}

	u := s3AccessLogUnmarshaler{buildInfo: component.BuildInfo{}}
	for name, benchmark := range benchmarks {
		logs := newLogFileContent(b, benchmark.nLogs)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = u.UnmarshalLogs(logs)
			}
		})
	}
}
