// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package fileexporter

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

type testMarshaller struct {
	content []byte
}

func (m *testMarshaller) MarshalTraces(ptrace.Traces) ([]byte, error) {
	return m.content, nil
}

func (m *testMarshaller) MarshalLogs(plog.Logs) ([]byte, error) {
	return m.content, nil
}

func (m *testMarshaller) MarshalMetrics(pmetric.Metrics) ([]byte, error) {
	return m.content, nil
}

func (m *testMarshaller) MarshalProfiles(pprofile.Profiles) ([]byte, error) {
	return m.content, nil
}

type groupingExporterTestCase struct {
	name               string
	conf               *Config
	traceUnmarshaler   ptrace.Unmarshaler
	logUnmarshaler     plog.Unmarshaler
	metricUnmarshaler  pmetric.Unmarshaler
	profileUnmarshaler pprofile.Unmarshaler
}

func groupingExporterTestCases() []groupingExporterTestCase {
	return []groupingExporterTestCase{
		{
			name: "json: default configuration",
			conf: &Config{
				FormatType: formatTypeJSON,
				Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled: true,
					// defaults:
					ResourceAttribute: defaultResourceAttribute,
					MaxOpenFiles:      defaultMaxOpenFiles,
				},
			},
			traceUnmarshaler:   &ptrace.JSONUnmarshaler{},
			logUnmarshaler:     &plog.JSONUnmarshaler{},
			metricUnmarshaler:  &pmetric.JSONUnmarshaler{},
			profileUnmarshaler: &pprofile.JSONUnmarshaler{},
		},
		{
			name: "json: compression configuration",
			conf: &Config{
				FormatType:  formatTypeJSON,
				Compression: compressionZSTD,
				Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled: true,
					// defaults:
					ResourceAttribute: defaultResourceAttribute,
					MaxOpenFiles:      defaultMaxOpenFiles,
				},
			},
			traceUnmarshaler:   &ptrace.JSONUnmarshaler{},
			logUnmarshaler:     &plog.JSONUnmarshaler{},
			metricUnmarshaler:  &pmetric.JSONUnmarshaler{},
			profileUnmarshaler: &pprofile.JSONUnmarshaler{},
		},
		{
			name: "Proto: default configuration",
			conf: &Config{
				FormatType: formatTypeProto,
				GroupBy: &GroupBy{
					Enabled: true,
					// defaults:
					ResourceAttribute: defaultResourceAttribute,
					MaxOpenFiles:      defaultMaxOpenFiles,
				},
			},
			traceUnmarshaler:   &ptrace.ProtoUnmarshaler{},
			logUnmarshaler:     &plog.ProtoUnmarshaler{},
			metricUnmarshaler:  &pmetric.ProtoUnmarshaler{},
			profileUnmarshaler: &pprofile.JSONUnmarshaler{},
		},
		{
			name: "Proto: compression configuration",
			conf: &Config{
				FormatType:  formatTypeProto,
				Compression: compressionZSTD,
				Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled: true,
					// defaults:
					ResourceAttribute: defaultResourceAttribute,
					MaxOpenFiles:      defaultMaxOpenFiles,
				},
			},
			traceUnmarshaler:   &ptrace.ProtoUnmarshaler{},
			logUnmarshaler:     &plog.ProtoUnmarshaler{},
			metricUnmarshaler:  &pmetric.ProtoUnmarshaler{},
			profileUnmarshaler: &pprofile.JSONUnmarshaler{},
		},
		{
			name: "json: max_open_files=1",
			conf: &Config{
				FormatType: formatTypeJSON,
				Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled:      true,
					MaxOpenFiles: 1,
					// defaults:
					ResourceAttribute: defaultResourceAttribute,
				},
			},
			traceUnmarshaler:   &ptrace.JSONUnmarshaler{},
			logUnmarshaler:     &plog.JSONUnmarshaler{},
			metricUnmarshaler:  &pmetric.JSONUnmarshaler{},
			profileUnmarshaler: &pprofile.JSONUnmarshaler{},
		},
	}
}

func TestGroupingFileTracesExporter(t *testing.T) {
	for _, tt := range groupingExporterTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.conf
			tmpDir := t.TempDir()
			conf.Path = tmpDir + "/*.log"
			zapCore, logs := observer.New(zap.DebugLevel)
			feI := newFileExporter(conf, zap.New(zapCore))
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testSpans := func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				testdata.GenerateTracesOneSpan().ResourceSpans().At(0).CopyTo(td.ResourceSpans().AppendEmpty())
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", "one")
				td.ResourceSpans().At(1).Resource().Attributes().PutStr("fileexporter.path_segment", ".././two/two")
				return td
			}
			td := testSpans()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeTraces(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupBy.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			// make sure the exporter did not modify any data
			assert.Equal(t, testSpans(), td)

			debugLogs := logs.FilterLevelExact(zap.DebugLevel)
			assert.Equal(t, 1, debugLogs.Len())
			assert.Equal(t, 0, logs.Len()-debugLogs.Len())

			pathResourceSpans := map[string][]ptrace.ResourceSpans{
				tmpDir + "/one.log":     {td.ResourceSpans().At(0)},
				tmpDir + "/two/two.log": {td.ResourceSpans().At(1)},
			}

			for path, wantResourceSpans := range pathResourceSpans {
				fi, err := os.Open(path)
				if len(wantResourceSpans) == 0 {
					assert.Error(t, err)
					continue
				}
				assert.NoError(t, err)
				br := bufio.NewReader(fi)
				for {
					buf, isEnd, err := func() ([]byte, bool, error) {
						if gfe.marshaller.formatType == formatTypeJSON && gfe.marshaller.compression == "" {
							return readJSONMessage(br)
						}
						return readMessageFromStream(br)
					}()
					assert.NoError(t, err)
					if isEnd {
						break
					}
					decoder := buildUnCompressor(gfe.marshaller.compression)
					buf, err = decoder(buf)
					assert.NoError(t, err)
					got, err := tt.traceUnmarshaler.UnmarshalTraces(buf)
					assert.NoError(t, err)

					gotResourceSpans := make([]ptrace.ResourceSpans, 0)
					for i := 0; i < got.ResourceSpans().Len(); i++ {
						gotResourceSpans = append(gotResourceSpans, got.ResourceSpans().At(i))
					}

					assert.Equal(t, wantResourceSpans, gotResourceSpans)
				}
				fi.Close()
			}
		})
	}
}

func TestGroupingFileLogsExporter(t *testing.T) {
	for _, tt := range groupingExporterTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.conf
			tmpDir := t.TempDir()
			conf.Path = tmpDir + "/*.log"
			zapCore, logs := observer.New(zap.DebugLevel)
			feI := newFileExporter(conf, zap.New(zapCore))
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testLogs := func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResource()
				testdata.GenerateLogsOneLogRecord().ResourceLogs().At(0).CopyTo(td.ResourceLogs().AppendEmpty())
				testdata.GenerateLogsOneLogRecord().ResourceLogs().At(0).CopyTo(td.ResourceLogs().AppendEmpty())
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", "one")
				td.ResourceLogs().At(1).Resource().Attributes().PutStr("fileexporter.path_segment", ".././two/two")
				return td
			}
			td := testLogs()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeLogs(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupBy.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			// make sure the exporter did not modify any data
			assert.Equal(t, testLogs(), td)

			debugLogs := logs.FilterLevelExact(zap.DebugLevel)
			assert.Equal(t, 1, debugLogs.Len())
			assert.Equal(t, 0, logs.Len()-debugLogs.Len())

			pathResourceLogs := map[string][]plog.ResourceLogs{
				tmpDir + "/one.log":     {td.ResourceLogs().At(0)},
				tmpDir + "/two/two.log": {td.ResourceLogs().At(1)},
			}

			for path, wantResourceLogs := range pathResourceLogs {
				fi, err := os.Open(path)
				if len(wantResourceLogs) == 0 {
					assert.Error(t, err)
					continue
				}
				assert.NoError(t, err)
				br := bufio.NewReader(fi)
				for {
					buf, isEnd, err := func() ([]byte, bool, error) {
						if gfe.marshaller.formatType == formatTypeJSON && gfe.marshaller.compression == "" {
							return readJSONMessage(br)
						}
						return readMessageFromStream(br)
					}()
					assert.NoError(t, err)
					if isEnd {
						break
					}
					decoder := buildUnCompressor(gfe.marshaller.compression)
					buf, err = decoder(buf)
					assert.NoError(t, err)
					got, err := tt.logUnmarshaler.UnmarshalLogs(buf)
					assert.NoError(t, err)

					gotResourceLogs := make([]plog.ResourceLogs, 0)
					for i := 0; i < got.ResourceLogs().Len(); i++ {
						gotResourceLogs = append(gotResourceLogs, got.ResourceLogs().At(i))
					}

					assert.Equal(t, wantResourceLogs, gotResourceLogs)
				}
				fi.Close()
			}
		})
	}
}

func TestGroupingFileMetricsExporter(t *testing.T) {
	for _, tt := range groupingExporterTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.conf
			tmpDir := t.TempDir()
			conf.Path = tmpDir + "/*.log"

			zapCore, logs := observer.New(zap.DebugLevel)
			feI := newFileExporter(conf, zap.New(zapCore))
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testMetrics := func() pmetric.Metrics {
				td := testdata.GenerateMetricsTwoMetrics()
				testdata.GenerateMetricsOneCounterOneSummaryMetrics().ResourceMetrics().At(0).CopyTo(td.ResourceMetrics().AppendEmpty())
				testdata.GenerateMetricsOneMetricNoAttributes().ResourceMetrics().At(0).CopyTo(td.ResourceMetrics().AppendEmpty())
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", "one")
				td.ResourceMetrics().At(1).Resource().Attributes().PutStr("fileexporter.path_segment", ".././two/two")
				return td
			}
			td := testMetrics()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeMetrics(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupBy.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			// make sure the exporter did not modify any data
			assert.Equal(t, testMetrics(), td)

			debugLogs := logs.FilterLevelExact(zap.DebugLevel)
			assert.Equal(t, 1, debugLogs.Len())
			assert.Equal(t, 0, logs.Len()-debugLogs.Len())

			pathResourceMetrics := map[string][]pmetric.ResourceMetrics{
				tmpDir + "/one.log":     {td.ResourceMetrics().At(0)},
				tmpDir + "/two/two.log": {td.ResourceMetrics().At(1)},
			}

			for path, wantResourceMetrics := range pathResourceMetrics {
				fi, err := os.Open(path)
				if len(wantResourceMetrics) == 0 {
					assert.Error(t, err)
					continue
				}
				assert.NoError(t, err)
				br := bufio.NewReader(fi)
				for {
					buf, isEnd, err := func() ([]byte, bool, error) {
						if gfe.marshaller.formatType == formatTypeJSON && gfe.marshaller.compression == "" {
							return readJSONMessage(br)
						}
						return readMessageFromStream(br)
					}()
					assert.NoError(t, err)
					if isEnd {
						break
					}
					decoder := buildUnCompressor(gfe.marshaller.compression)
					buf, err = decoder(buf)
					assert.NoError(t, err)
					got, err := tt.metricUnmarshaler.UnmarshalMetrics(buf)
					assert.NoError(t, err)

					gotResourceMetrics := make([]pmetric.ResourceMetrics, 0)
					for i := 0; i < got.ResourceMetrics().Len(); i++ {
						gotResourceMetrics = append(gotResourceMetrics, got.ResourceMetrics().At(i))
					}

					assert.Equal(t, wantResourceMetrics, gotResourceMetrics)
				}
				fi.Close()
			}
		})
	}
}

func TestFullPath(t *testing.T) {
	tests := []struct {
		prefix      string
		pathSegment string
		suffix      string
		want        string
	}{
		// good actor
		{prefix: "/", pathSegment: "filename", suffix: ".json", want: "/filename.json"},
		{prefix: "/", pathSegment: "/dir/filename", suffix: ".json", want: "/dir/filename.json"},
		{prefix: "/dir", pathSegment: "dirsuffix/filename", suffix: ".json", want: "/dirdirsuffix/filename.json"},
		{prefix: "/dir", pathSegment: "/subdir/filename", suffix: ".json", want: "/dir/subdir/filename.json"},
		{prefix: "/dir", pathSegment: "./filename", suffix: ".json", want: "/dir/filename.json"},
		{prefix: "/dir", pathSegment: "/subdir/", suffix: "filename.json", want: "/dir/subdir/filename.json"},
		{prefix: "/dir/", pathSegment: "subdir", suffix: "/filename.json", want: "/dir/subdir/filename.json"},
		{prefix: "/dir", pathSegment: "", suffix: "filename.json", want: "/dirfilename.json"},
		{prefix: "/dir/", pathSegment: "", suffix: "filename.json", want: "/dir/filename.json"},
		{prefix: "/dir/", pathSegment: "subdir/strangebutok/../", suffix: "filename.json", want: "/dir/subdir/filename.json"},
		{prefix: "/dir", pathSegment: "dirsuffix/strangebutok/../", suffix: "filename.json", want: "/dirdirsuffix/filename.json"},

		// bad actor
		{prefix: "/dir", pathSegment: "../etc/attack", suffix: ".json", want: "/dir/etc/attack.json"},
		{prefix: "/dir", pathSegment: "../etc/attack", suffix: "/filename.json", want: "/dir/etc/attack/filename.json"},
		{prefix: "/dir", pathSegment: "dirsuffix/../etc/attack", suffix: ".json", want: "/dir/etc/attack.json"},
		{prefix: "/dir", pathSegment: "dirsuffix/../../etc/attack", suffix: ".json", want: "/dir/etc/attack.json"},
		{prefix: "/dir", pathSegment: "dirsuffix/../../etc/attack", suffix: ".json", want: "/dir/etc/attack.json"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s + %s + %s", tc.prefix, tc.pathSegment, tc.suffix), func(t *testing.T) {
			e := &groupingFileExporter{
				pathPrefix: cleanPathPrefix(tc.prefix),
				pathSuffix: tc.suffix,
			}

			assert.Equal(t, tc.want, e.fullPath(tc.pathSegment))
		})
	}
}

func BenchmarkExporters(b *testing.B) {
	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "default",
			conf: &Config{
				Path:       tempFileName(b),
				FormatType: formatTypeJSON,
			},
		},
		{
			name: "grouping, 100 writers",
			conf: &Config{
				Path:       b.TempDir() + "/*",
				FormatType: formatTypeJSON,
				GroupBy: &GroupBy{
					Enabled:      true,
					MaxOpenFiles: 100,
				},
			},
		},
		{
			name: "grouping, 99 writers",
			conf: &Config{
				Path:       b.TempDir() + "/*",
				FormatType: formatTypeJSON,
				GroupBy: &GroupBy{
					Enabled:      true,
					MaxOpenFiles: 99,
				},
			},
		},
		{
			name: "grouping, 1 writer",
			conf: &Config{
				Path:       b.TempDir() + "/*",
				FormatType: formatTypeJSON,
				GroupBy: &GroupBy{
					Enabled:      true,
					MaxOpenFiles: 1,
				},
			},
		},
	}

	var traces []ptrace.Traces
	var logs []plog.Logs
	for i := 0; i < 100; i++ {
		td := testdata.GenerateTracesTwoSpansSameResource()
		td.ResourceSpans().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", fmt.Sprintf("file%d", i))
		traces = append(traces, td)

		ld := testdata.GenerateLogsTwoLogRecordsSameResource()
		ld.ResourceLogs().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", fmt.Sprintf("file%d", i))
		logs = append(logs, ld)
	}
	for _, tc := range tests {
		fe := newFileExporter(tc.conf, zap.NewNop())

		// remove marshaling time from the benchmark
		tm := &testMarshaller{content: bytes.Repeat([]byte{'a'}, 512)}
		marshaller := &marshaller{
			tracesMarshaler:  tm,
			metricsMarshaler: tm,
			logsMarshaler:    tm,
			compression:      "",
			compressor:       noneCompress,
			formatType:       "test",
		}
		switch fExp := fe.(type) {
		case *fileExporter:
			fExp.marshaller = marshaller
		case *groupingFileExporter:
			fExp.marshaller = marshaller
		}

		require.NoError(b, fe.Start(context.Background(), componenttest.NewNopHost()))

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				require.NoError(b, fe.consumeTraces(ctx, traces[i%len(traces)]))
				require.NoError(b, fe.consumeLogs(ctx, logs[i%len(logs)]))
			}
		})

		assert.NoError(b, fe.Shutdown(context.Background()))
	}
}
