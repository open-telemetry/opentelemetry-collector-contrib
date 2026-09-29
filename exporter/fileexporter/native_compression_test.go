// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func setNativeCompressionFeatureGate(tb testing.TB, enabled bool) {
	prev := metadata.ExporterFileNativeCompressionFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ExporterFileNativeCompressionFeatureGate.ID(), enabled))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ExporterFileNativeCompressionFeatureGate.ID(), prev))
	})
}

func TestNativeZstdCompression(t *testing.T) {
	setNativeCompressionFeatureGate(t, true)

	path := filepath.Join(t.TempDir(), "telemetry.log.zst")
	conf := &Config{
		Path:        path,
		FormatType:  formatTypeProto,
		Compression: compressionZSTD,
		CompressionParams: configcompression.CompressionParams{
			Level: 3,
		},
	}

	fe := &fileExporter{conf: conf}
	td := testdata.GenerateTracesTwoSpansSameResource()

	require.NoError(t, fe.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.Shutdown(t.Context()))

	decompressed := decompressZstd(t, path)
	require.NotEmpty(t, decompressed)

	// Verify proto messages can be read from decompressed data
	br := bufio.NewReader(bytes.NewReader(decompressed))
	unmarshaler := &ptrace.ProtoUnmarshaler{}
	count := 0
	for {
		buf, isEnd, err := readMessageFromStream(br)
		require.NoError(t, err)
		if isEnd {
			break
		}
		got, err := unmarshaler.UnmarshalTraces(buf)
		require.NoError(t, err)
		require.Equal(t, td, got)
		count++
	}
	require.Equal(t, 2, count)
}

func TestNativeZstdCompression_JSONFormat(t *testing.T) {
	setNativeCompressionFeatureGate(t, true)

	path := filepath.Join(t.TempDir(), "telemetry.log.zst")
	conf := &Config{
		Path:        path,
		FormatType:  formatTypeJSON,
		Compression: compressionZSTD,
	}

	fe := &fileExporter{conf: conf}
	td := testdata.GenerateTracesTwoSpansSameResource()

	require.NoError(t, fe.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.Shutdown(t.Context()))

	decompressed := decompressZstd(t, path)

	// With native compression + JSON, data should be newline-delimited JSON
	br := bufio.NewReader(bytes.NewReader(decompressed))
	unmarshaler := &ptrace.JSONUnmarshaler{}
	count := 0
	for {
		buf, isEnd, err := readJSONMessage(br)
		require.NoError(t, err)
		if isEnd {
			break
		}
		got, err := unmarshaler.UnmarshalTraces(buf)
		require.NoError(t, err)
		require.Equal(t, td, got)
		count++
	}
	require.Equal(t, 2, count)
}

func TestNativeZstdCompression_WithNativeTools(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd command not available, skipping native tool test")
	}

	setNativeCompressionFeatureGate(t, true)

	path := filepath.Join(t.TempDir(), "telemetry.log.zst")
	conf := &Config{
		Path:        path,
		FormatType:  formatTypeProto,
		Compression: compressionZSTD,
	}

	fe := &fileExporter{conf: conf}
	td := testdata.GenerateTracesTwoSpansSameResource()

	require.NoError(t, fe.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.Shutdown(t.Context()))

	// Decompress with native zstd command
	outputPath := filepath.Join(t.TempDir(), "decompressed.log")
	cmd := exec.Command("zstd", "-d", path, "-o", outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "native zstd decompression should succeed: %s", stderr.String())

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.Positive(t, info.Size())

	// Verify integrity
	var stderrTest bytes.Buffer
	cmd = exec.Command("zstd", "-t", path)
	cmd.Stderr = &stderrTest
	err = cmd.Run()
	require.NoError(t, err, "zstd integrity test should pass: %s", stderrTest.String())
}

func TestLegacyCompression_WhenFeatureGateDisabled(t *testing.T) {
	setNativeCompressionFeatureGate(t, false)

	path := filepath.Join(t.TempDir(), "telemetry_legacy.log")
	conf := &Config{
		Path:        path,
		FormatType:  formatTypeProto,
		Compression: compressionZSTD,
	}

	feI := newFileExporter(conf, zap.NewNop())
	require.IsType(t, &fileExporter{}, feI)
	fe := feI.(*fileExporter)

	td := testdata.GenerateTracesTwoSpansSameResource()
	require.NoError(t, fe.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, fe.consumeTraces(t.Context(), td))
	require.NoError(t, fe.Shutdown(t.Context()))

	// Verify file uses legacy format: try to decompress with native zstd
	// It should fail because it's not a native zstd format
	if _, err := exec.LookPath("zstd"); err == nil {
		cmd := exec.Command("zstd", "-t", path)
		err := cmd.Run()
		require.Error(t, err, "legacy format should not be decompressable with native zstd")
	}

	// Verify the legacy format is readable with the existing message stream reader
	fi, err := os.Open(path)
	require.NoError(t, err)
	defer fi.Close()

	br := bufio.NewReader(fi)
	buf, isEnd, err := readMessageFromStream(br)
	require.NoError(t, err)
	require.False(t, isEnd)

	// Decompress with legacy decompressor
	decompressedBuf, err := decompress(buf)
	require.NoError(t, err)

	unmarshaler := &ptrace.ProtoUnmarshaler{}
	got, err := unmarshaler.UnmarshalTraces(decompressedBuf)
	require.NoError(t, err)
	require.Equal(t, td, got)
}

func TestNativeZstdCompression_WithRotation(t *testing.T) {
	setNativeCompressionFeatureGate(t, true)

	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.log.zst")
	conf := &Config{
		Path:        path,
		FormatType:  formatTypeProto,
		Compression: compressionZSTD,
		Rotation: &Rotation{
			MaxMegabytes: 1, // Small to trigger rotation
			MaxBackups:   3,
		},
	}

	fe := &fileExporter{conf: conf}
	td := testdata.GenerateTracesTwoSpansSameResource()

	require.NoError(t, fe.Start(t.Context(), componenttest.NewNopHost()))

	// Write enough data to trigger rotation
	for range 100 {
		require.NoError(t, fe.consumeTraces(t.Context(), td))
	}

	require.NoError(t, fe.Shutdown(t.Context()))

	// Collect all files in the directory (active + rotated backups)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "expected at least one output file")

	totalTraces := 0
	unmarshaler := &ptrace.ProtoUnmarshaler{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		compressed, err := os.ReadFile(filePath)
		require.NoError(t, err, "failed to read file %s", entry.Name())
		if len(compressed) == 0 {
			continue
		}

		// Every file must be valid zstd
		reader, err := zstd.NewReader(bytes.NewReader(compressed))
		require.NoError(t, err, "file %s is not valid zstd", entry.Name())

		decompressed, err := io.ReadAll(reader)
		reader.Close()
		require.NoError(t, err, "failed to decompress file %s", entry.Name())
		require.NotEmpty(t, decompressed, "decompressed file %s is empty", entry.Name())

		// Verify proto messages can be read from decompressed data
		br := bufio.NewReader(bytes.NewReader(decompressed))
		for {
			buf, isEnd, err := readMessageFromStream(br)
			require.NoError(t, err, "failed to read message from file %s", entry.Name())
			if isEnd {
				break
			}
			_, err = unmarshaler.UnmarshalTraces(buf)
			require.NoError(t, err, "failed to unmarshal traces from file %s", entry.Name())
			totalTraces++
		}
	}

	require.Equal(t, 100, totalTraces, "expected all 100 traces to be recoverable across all files")
}

// binaryLogsEncoding marshals log bodies without advertising stream decoding,
// standing in for encodings whose output may be binary, such as otlp_encoding.
type binaryLogsEncoding struct{}

func (binaryLogsEncoding) Start(context.Context, component.Host) error { return nil }
func (binaryLogsEncoding) Shutdown(context.Context) error              { return nil }

func (binaryLogsEncoding) MarshalLogs(ld plog.Logs) ([]byte, error) {
	var buf bytes.Buffer
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				buf.WriteString(lrs.At(k).Body().AsString())
			}
		}
	}
	return buf.Bytes(), nil
}

// textLogsEncoding is binaryLogsEncoding plus stream decoding, standing in for textencodingextension.
type textLogsEncoding struct{ binaryLogsEncoding }

func (textLogsEncoding) NewLogsDecoder(io.Reader, ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return nil, errors.New("not implemented")
}

func logsWithBody(body string) plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	return ld
}

// Under native compression, a stream-decodable encoding on the default `json` format is
// newline-delimited so it survives zstdcat and grep. Every other encoding keeps length prefixes. See #49328.
func TestNativeCompression_EncodingFraming(t *testing.T) {
	const prefixed = "\x00\x00\x00\x08line-one\x00\x00\x00\x08line-two"
	tests := []struct {
		name       string
		formatType string
		encoding   component.Component
		expected   string
	}{
		{
			name:       "text encoding with json is newline delimited",
			formatType: formatTypeJSON,
			encoding:   textLogsEncoding{},
			expected:   "line-one\nline-two\n",
		},
		{
			name:       "text encoding with proto keeps length prefixes",
			formatType: formatTypeProto,
			encoding:   textLogsEncoding{},
			expected:   prefixed,
		},
		{
			name:       "binary encoding with json keeps length prefixes",
			formatType: formatTypeJSON,
			encoding:   binaryLogsEncoding{},
			expected:   prefixed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setNativeCompressionFeatureGate(t, true)

			encID := component.MustNewID("testencoding")
			path := filepath.Join(t.TempDir(), "app.log.zst")
			conf := &Config{
				Path:              path,
				FormatType:        test.formatType,
				Encoding:          &encID,
				Compression:       compressionZSTD,
				CompressionParams: configcompression.CompressionParams{Level: 3},
			}

			host := hostWithEncoding{map[component.ID]component.Component{encID: test.encoding}}
			fe := &fileExporter{conf: conf}
			require.NoError(t, fe.Start(t.Context(), host))
			require.NoError(t, fe.consumeLogs(t.Context(), logsWithBody("line-one")))
			require.NoError(t, fe.consumeLogs(t.Context(), logsWithBody("line-two")))
			require.NoError(t, fe.Shutdown(t.Context()))

			require.Equal(t, test.expected, string(decompressZstd(t, path)))
		})
	}
}

func decompressZstd(t *testing.T, path string) []byte {
	t.Helper()
	compressed, err := os.ReadFile(path)
	require.NoError(t, err)
	reader, err := zstd.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer reader.Close()
	decompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return decompressed
}
