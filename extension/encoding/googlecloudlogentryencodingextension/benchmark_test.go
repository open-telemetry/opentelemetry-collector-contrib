// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func newFileContent(filename string, nLogs int) ([]byte, error) {
	log, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var dest bytes.Buffer
	if err := gojson.Compact(&dest, log); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for i := 0; i < nLogs; i++ {
		buf.Write(dest.Bytes())
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func BenchmarkUnmarshaler(b *testing.B) {
	ex := &ext{}
	testCases := []int{1, 1_000, 100_000}

	for _, n := range testCases {
		content, err := newFileContent("testdata/log_entry.json", n)
		require.NoError(b, err)
		b.Run(fmt.Sprintf("unmarshal %d logs", n), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := ex.UnmarshalLogs(content)
				require.NoError(b, err)
			}
		})
		b.Run(fmt.Sprintf("decode %d logs", n), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				decoder, err := ex.NewLogsDecoder(bytes.NewReader(content))
				require.NoError(b, err)

				// loading everything into memory
				logs := plog.NewLogs()
				for {
					err = decoder.DecodeLogs(logs)
					if errors.Is(err, io.EOF) {
						break
					}
					require.NoError(b, err)
				}
			}
		})
	}
}
