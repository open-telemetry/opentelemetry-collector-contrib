// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkfirewall

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// readAndCompressLogFileForBenchmark reads and compresses log file for benchmarking
func readAndCompressLogFileForBenchmark(b *testing.B, dir, file string) []byte {
	b.Helper()
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(b, err)
	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(b, err)

	var compressedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedData)
	_, err = gzipWriter.Write(compacted.Bytes())
	require.NoError(b, err)
	err = gzipWriter.Close()
	require.NoError(b, err)
	return compressedData.Bytes()
}

func BenchmarkUnmarshalNetworkFirewallAlertLog(b *testing.B) {
	factory := NewNetworkFirewallLogUnmarshalerFactory(component.BuildInfo{})
	data := readAndCompressLogFileForBenchmark(b, "testdata", "valid_alert_log.json")

	for b.Loop() {
		decoder, err := factory(bytes.NewReader(data), encoding.StreamDecoderOptions{})
		if err != nil {
			b.Fatal(err)
		}
		logs := plog.NewLogs()
		err = decoder.Decode(context.Background(), logs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalNetworkFirewallFlowLog(b *testing.B) {
	factory := NewNetworkFirewallLogUnmarshalerFactory(component.BuildInfo{})
	data := readAndCompressLogFileForBenchmark(b, "testdata", "valid_flow_log.json")

	for b.Loop() {
		decoder, err := factory(bytes.NewReader(data), encoding.StreamDecoderOptions{})
		if err != nil {
			b.Fatal(err)
		}
		logs := plog.NewLogs()
		err = decoder.Decode(context.Background(), logs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalNetworkFirewallTLSLog(b *testing.B) {
	factory := NewNetworkFirewallLogUnmarshalerFactory(component.BuildInfo{})
	data := readAndCompressLogFileForBenchmark(b, "testdata", "valid_tls_log.json")

	for b.Loop() {
		decoder, err := factory(bytes.NewReader(data), encoding.StreamDecoderOptions{})
		if err != nil {
			b.Fatal(err)
		}
		logs := plog.NewLogs()
		err = decoder.Decode(context.Background(), logs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
