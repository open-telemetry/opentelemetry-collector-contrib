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
	"go.opentelemetry.io/collector/pdata/ptrace"

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

type groupingExporterTestCase struct {
	name              string
	conf              *Config
	traceUnmarshaler  ptrace.Unmarshaler
	logUnmarshaler    plog.Unmarshaler
	metricUnmarshaler pmetric.Unmarshaler
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
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
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
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
		},
		{
			name: "Proto: default configuration",
			conf: &Config{
				FormatType: formatTypeProto,
				GroupBy: &GroupBy{
					Enabled: true,
					// defaults:
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.ProtoUnmarshaler{},
			logUnmarshaler:    &plog.ProtoUnmarshaler{},
			metricUnmarshaler: &pmetric.ProtoUnmarshaler{},
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
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.ProtoUnmarshaler{},
			logUnmarshaler:    &plog.ProtoUnmarshaler{},
			metricUnmarshaler: &pmetric.ProtoUnmarshaler{},
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
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
		},
		{
			name: "json: delete_sub_path_resource_attribute=true",
			conf: &Config{
				FormatType: formatTypeJSON,
				Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled:                        true,
					DeleteSubPathResourceAttribute: true,
					// defaults:
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
		},
		{
			name: "json: discard_if_attribute_not_found=true",
			conf: &Config{
				FormatType: formatTypeJSON,
				Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
				GroupBy: &GroupBy{
					Enabled:                    true,
					DiscardIfAttributeNotFound: true,
					// defaults:
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             defaultMaxOpenFiles,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
			traceUnmarshaler:  &ptrace.JSONUnmarshaler{},
			logUnmarshaler:    &plog.JSONUnmarshaler{},
			metricUnmarshaler: &pmetric.JSONUnmarshaler{},
		},
	}
}

func TestGroupingFileTracesExporter(t *testing.T) {
	for _, tt := range groupingExporterTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.conf
			tmpDir := t.TempDir()
			conf.Path = tmpDir + "/*.log"
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
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

			removeAttr := func(rSpans ptrace.ResourceSpans) ptrace.ResourceSpans {
				if conf.GroupBy.DeleteSubPathResourceAttribute {
					rSpans.Resource().Attributes().Remove("fileexporter.path_segment")
				}
				return rSpans
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testSpans()
			pathResourceSpans := map[string][]ptrace.ResourceSpans{
				tmpDir + "/one.log":     {removeAttr(td.ResourceSpans().At(0))},
				tmpDir + "/two/two.log": {removeAttr(td.ResourceSpans().At(1))},
				tmpDir + "/MISSING.log": {removeAttr(td.ResourceSpans().At(2))},
			}

			if conf.GroupBy.DiscardIfAttributeNotFound {
				pathResourceSpans[tmpDir+"/MISSING.log"] = nil
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

					assert.EqualValues(t, wantResourceSpans, gotResourceSpans)
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
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
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

			removeAttr := func(rLogs plog.ResourceLogs) plog.ResourceLogs {
				if conf.GroupBy.DeleteSubPathResourceAttribute {
					rLogs.Resource().Attributes().Remove("fileexporter.path_segment")
				}
				return rLogs
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testLogs()
			pathResourceLogs := map[string][]plog.ResourceLogs{
				tmpDir + "/one.log":     {removeAttr(td.ResourceLogs().At(0))},
				tmpDir + "/two/two.log": {removeAttr(td.ResourceLogs().At(1))},
				tmpDir + "/MISSING.log": {removeAttr(td.ResourceLogs().At(2))},
			}

			if conf.GroupBy.DiscardIfAttributeNotFound {
				pathResourceLogs[tmpDir+"/MISSING.log"] = []plog.ResourceLogs{}
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

					assert.EqualValues(t, wantResourceLogs, gotResourceLogs)
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
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
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

			removeAttr := func(rMetrics pmetric.ResourceMetrics) pmetric.ResourceMetrics {
				if conf.GroupBy.DeleteSubPathResourceAttribute {
					rMetrics.Resource().Attributes().Remove("fileexporter.path_segment")
				}
				return rMetrics
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testMetrics()
			pathResourceMetrics := map[string][]pmetric.ResourceMetrics{
				tmpDir + "/one.log":     {removeAttr(td.ResourceMetrics().At(0))},
				tmpDir + "/two/two.log": {removeAttr(td.ResourceMetrics().At(1))},
				tmpDir + "/MISSING.log": {removeAttr(td.ResourceMetrics().At(2))},
			}

			if conf.GroupBy.DiscardIfAttributeNotFound {
				pathResourceMetrics[tmpDir+"/MISSING.log"] = []pmetric.ResourceMetrics{}
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

					assert.EqualValues(t, wantResourceMetrics, gotResourceMetrics)
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

func TestGroupingFileExporterErrors(t *testing.T) {
	fe, err := newFileExporter(&Config{
		Path:       t.TempDir() + "/*",
		FormatType: formatTypeJSON,
		GroupBy: &GroupBy{
			Enabled:               true,
			AutoCreateDirectories: false,
			// default:
			SubPathResourceAttribute: defaultSubPathResourceAttribute,
			DefaultSubPath:           defaultSubPath,
			MaxOpenFiles:             defaultMaxOpenFiles,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	ld.ResourceLogs().At(0).Resource().Attributes().PutStr("fileexporter.path_segment", "one/two")

	// Cannot create file (directory doesn't exist)
	assert.Error(t, fe.consumeLogs(context.Background(), ld))
	assert.NoError(t, fe.Shutdown(context.Background()))
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
					Enabled:                  true,
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             100,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
		},
		{
			name: "grouping, 99 writers",
			conf: &Config{
				Path:       b.TempDir() + "/*",
				FormatType: formatTypeJSON,
				GroupBy: &GroupBy{
					Enabled:                  true,
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             99,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
		},
		{
			name: "grouping, 1 writer",
			conf: &Config{
				Path:       b.TempDir() + "/*",
				FormatType: formatTypeJSON,
				GroupBy: &GroupBy{
					Enabled:                  true,
					SubPathResourceAttribute: defaultSubPathResourceAttribute,
					MaxOpenFiles:             1,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
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
		fe, err := newFileExporter(tc.conf)
		require.NoError(b, err)

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
