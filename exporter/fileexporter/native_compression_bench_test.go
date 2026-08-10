// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Feature gate is controlled via NATIVE_COMPRESSION env var so that
// benchmark names stay identical between runs for benchstat comparison.
//
//	# Legacy:
//	go test -bench=BenchmarkZstdExport -benchmem -count=6 ./exporter/fileexporter/ > legacy.txt
//
//	# Native:
//	NATIVE_COMPRESSION=1 go test -bench=BenchmarkZstdExport -benchmem -count=6 ./exporter/fileexporter/ > native.txt
//
//	# Compare:
//	benchstat legacy.txt native.txt
func init() {
	if os.Getenv("NATIVE_COMPRESSION") != "" {
		_ = featuregate.GlobalRegistry().Set(metadata.ExporterFileNativeCompressionFeatureGate.ID(), true)
	}
}

func BenchmarkZstdExportTraces(b *testing.B) {
	for _, format := range []string{formatTypeProto, formatTypeJSON} {
		b.Run(format, func(b *testing.B) {
			benchExportTraces(b, format, compressionZSTD, 0, testdata.GenerateTracesTwoSpansSameResource())
		})
	}
}

func BenchmarkZstdExportTracesLarge(b *testing.B) {
	td := generateLargeTraces()
	for _, format := range []string{formatTypeProto, formatTypeJSON} {
		b.Run(format, func(b *testing.B) {
			benchExportTraces(b, format, compressionZSTD, 0, td)
		})
	}
}

func BenchmarkZstdExportLogs(b *testing.B) {
	ld := generateLargeLogs()
	for _, format := range []string{formatTypeProto, formatTypeJSON} {
		b.Run(format, func(b *testing.B) {
			benchExportLogs(b, format, compressionZSTD, 0, ld)
		})
	}
}

func BenchmarkZstdExportLevels(b *testing.B) {
	td := generateLargeTraces()
	for _, level := range []configcompression.Level{1, 3, 6, 11} {
		b.Run(fmt.Sprintf("level_%d", level), func(b *testing.B) {
			benchExportTraces(b, formatTypeProto, compressionZSTD, level, td)
		})
	}
}

func BenchmarkNoCompression(b *testing.B) {
	for _, format := range []string{formatTypeProto, formatTypeJSON} {
		b.Run(format, func(b *testing.B) {
			benchExportTraces(b, format, "", 0, testdata.GenerateTracesTwoSpansSameResource())
		})
	}
}

func benchExportTraces(b *testing.B, format, compression string, level configcompression.Level, td ptrace.Traces) {
	b.Helper()

	fe := &fileExporter{conf: &Config{
		Path:        filepath.Join(b.TempDir(), "bench.out"),
		FormatType:  format,
		Compression: compression,
		CompressionParams: configcompression.CompressionParams{
			Level: level,
		},
	}}
	require.NoError(b, fe.Start(b.Context(), componenttest.NewNopHost()))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := fe.consumeTraces(b.Context(), td); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	require.NoError(b, fe.Shutdown(b.Context()))
	info, err := os.Stat(fe.conf.Path)
	require.NoError(b, err)
	b.ReportMetric(float64(info.Size())/float64(b.N), "output-bytes/op")
}

func benchExportLogs(b *testing.B, format, compression string, level configcompression.Level, ld plog.Logs) {
	b.Helper()

	fe := &fileExporter{conf: &Config{
		Path:        filepath.Join(b.TempDir(), "bench.out"),
		FormatType:  format,
		Compression: compression,
		CompressionParams: configcompression.CompressionParams{
			Level: level,
		},
	}}
	require.NoError(b, fe.Start(b.Context(), componenttest.NewNopHost()))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := fe.consumeLogs(b.Context(), ld); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	require.NoError(b, fe.Shutdown(b.Context()))
	info, err := os.Stat(fe.conf.Path)
	require.NoError(b, err)
	b.ReportMetric(float64(info.Size())/float64(b.N), "output-bytes/op")
}

func generateLargeTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	for range 10 {
		src := testdata.GenerateTracesTwoSpansSameResource()
		src.ResourceSpans().MoveAndAppendTo(td.ResourceSpans())
	}
	return td
}

func generateLargeLogs() plog.Logs {
	ld := plog.NewLogs()
	for range 10 {
		src := testdata.GenerateLogsTwoLogRecordsSameResource()
		src.ResourceLogs().MoveAndAppendTo(ld.ResourceLogs())
	}
	return ld
}
