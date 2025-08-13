// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs

import (
	"bytes"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newBuf creates an azure resource log with
// as many records as defined in nRecords
func newBuf(b *testing.B, nRecords int) []byte {
	rec := azureLogRecord{
		Time:          "2024-04-24T12:06:12.0000000Z",
		ResourceID:    "/test",
		Category:      "AppServiceAppLogs",
		OperationName: "AppLog",
	}

	data, err := gojson.Marshal(rec)
	require.NoError(b, err)

	buf := bytes.NewBuffer(make([]byte, 0, nRecords*(len(data))))
	buf.WriteString(`{"records": [`)
	for i := 0; i < nRecords; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(data)
	}
	buf.WriteString(`]}`)

	return buf.Bytes()
}

func BenchmarkUnmarshalLogs(b *testing.B) {
	tests := map[string]struct {
		nRecords int
	}{
		"1_record": {
			nRecords: 1,
		},
		"100_record": {
			nRecords: 100,
		},
		"1000_record": {
			nRecords: 1_000,
		},
	}

	u := ResourceLogsUnmarshaler{
		Version: "test",
		Logger:  zap.NewNop(),
	}
	for name, test := range tests {
		buf := newBuf(b, test.nRecords)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := u.UnmarshalLogs(buf)
				require.NoError(b, err)
			}
		})
	}
}
