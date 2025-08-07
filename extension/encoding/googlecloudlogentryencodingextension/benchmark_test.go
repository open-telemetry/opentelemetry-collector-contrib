// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"bytes"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func BenchmarkTest(b *testing.B) {
	name := "testdata/log_entry.json"
	data, err := os.ReadFile(name)
	require.NoError(b, err)

	var dest bytes.Buffer
	err = gojson.Compact(&dest, data)
	require.NoError(b, err)

	ex := &ext{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ex.UnmarshalLogs(dest.Bytes())
		require.NoError(b, err)
	}
}
