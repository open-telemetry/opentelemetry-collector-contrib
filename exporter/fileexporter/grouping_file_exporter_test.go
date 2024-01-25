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

func TestGroupingFileTracesExporter(t *testing.T) {
	type args struct {
		conf        *Config
		unmarshaler ptrace.Unmarshaler
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "json: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.JSONUnmarshaler{},
			},
		},
		{
			name: "json: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeJSON,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.JSONUnmarshaler{},
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeProto,
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.ProtoUnmarshaler{},
			},
		},
		{
			name: "Proto: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeProto,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.ProtoUnmarshaler{},
			},
		},
		{
			name: "json: max_open_files=1",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						MaxOpenFiles: 1,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.JSONUnmarshaler{},
			},
		},
		{
			name: "json: delete_sub_path_resource_attribute=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DeleteSubPathResourceAttribute: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.JSONUnmarshaler{},
			},
		},
		{
			name: "json: discard_if_attribute_not_found=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DiscardIfAttributeNotFound: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &ptrace.JSONUnmarshaler{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.args.conf
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testSpans := func() ptrace.Traces {
				td := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
				testdata.GenerateTracesOneSpan().ResourceSpans().At(0).CopyTo(td.ResourceSpans().AppendEmpty())
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("sub_path_attribute", "one")
				td.ResourceSpans().At(1).Resource().Attributes().PutStr("sub_path_attribute", ".././two/two")
				return td
			}
			td := testSpans()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeTraces(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupByAttribute.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			removeAttr := func(rSpans ptrace.ResourceSpans) ptrace.ResourceSpans {
				if conf.GroupByAttribute.DeleteSubPathResourceAttribute {
					rSpans.Resource().Attributes().Remove("sub_path_attribute")
				}
				return rSpans
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testSpans()
			pathResourceSpans := map[string][]ptrace.ResourceSpans{
				conf.Path + "/one":     {removeAttr(td.ResourceSpans().At(0))},
				conf.Path + "/two/two": {removeAttr(td.ResourceSpans().At(1))},
				conf.Path + "/MISSING": {removeAttr(td.ResourceSpans().At(2))},
			}

			if conf.GroupByAttribute.DiscardIfAttributeNotFound {
				pathResourceSpans[conf.Path+"/MISSING"] = []ptrace.ResourceSpans{}
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
					got, err := tt.args.unmarshaler.UnmarshalTraces(buf)
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
	type args struct {
		conf        *Config
		unmarshaler plog.Unmarshaler
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "json: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.JSONUnmarshaler{},
			},
		},
		{
			name: "json: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeJSON,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.JSONUnmarshaler{},
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeProto,
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.ProtoUnmarshaler{},
			},
		},
		{
			name: "Proto: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeProto,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.ProtoUnmarshaler{},
			},
		},
		{
			name: "json: max_open_files=1",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						MaxOpenFiles: 1,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.JSONUnmarshaler{},
			},
		},
		{
			name: "json: delete_sub_path_resource_attribute=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DeleteSubPathResourceAttribute: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.JSONUnmarshaler{},
			},
		},
		{
			name: "json: discard_if_attribute_not_found=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DiscardIfAttributeNotFound: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &plog.JSONUnmarshaler{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.args.conf
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testLogs := func() plog.Logs {
				td := testdata.GenerateLogsTwoLogRecordsSameResource()
				testdata.GenerateLogsOneLogRecord().ResourceLogs().At(0).CopyTo(td.ResourceLogs().AppendEmpty())
				testdata.GenerateLogsOneLogRecord().ResourceLogs().At(0).CopyTo(td.ResourceLogs().AppendEmpty())
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("sub_path_attribute", "one")
				td.ResourceLogs().At(1).Resource().Attributes().PutStr("sub_path_attribute", ".././two/two")
				return td
			}
			td := testLogs()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeLogs(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupByAttribute.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			removeAttr := func(rLogs plog.ResourceLogs) plog.ResourceLogs {
				if conf.GroupByAttribute.DeleteSubPathResourceAttribute {
					rLogs.Resource().Attributes().Remove("sub_path_attribute")
				}
				return rLogs
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testLogs()
			pathResourceLogs := map[string][]plog.ResourceLogs{
				conf.Path + "/one":     {removeAttr(td.ResourceLogs().At(0))},
				conf.Path + "/two/two": {removeAttr(td.ResourceLogs().At(1))},
				conf.Path + "/MISSING": {removeAttr(td.ResourceLogs().At(2))},
			}

			if conf.GroupByAttribute.DiscardIfAttributeNotFound {
				pathResourceLogs[conf.Path+"/MISSING"] = []plog.ResourceLogs{}
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
					got, err := tt.args.unmarshaler.UnmarshalLogs(buf)
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
	type args struct {
		conf        *Config
		unmarshaler pmetric.Unmarshaler
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "json: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.JSONUnmarshaler{},
			},
		},
		{
			name: "json: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeJSON,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.JSONUnmarshaler{},
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeProto,
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.ProtoUnmarshaler{},
			},
		},
		{
			name: "Proto: compression configuration",
			args: args{
				conf: &Config{
					Path:        t.TempDir(),
					FormatType:  formatTypeProto,
					Compression: compressionZSTD,
					Rotation:    &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.ProtoUnmarshaler{},
			},
		},
		{
			name: "json: max_open_files=1",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						MaxOpenFiles: 1,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.JSONUnmarshaler{},
			},
		},
		{
			name: "json: delete_sub_path_resource_attribute=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DeleteSubPathResourceAttribute: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.JSONUnmarshaler{},
			},
		},
		{
			name: "json: discard_if_attribute_not_found=true",
			args: args{
				conf: &Config{
					Path:       t.TempDir(),
					FormatType: formatTypeJSON,
					Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
					GroupByAttribute: &GroupByAttribute{
						DiscardIfAttributeNotFound: true,
						// defaults:
						SubPathResourceAttribute: "sub_path_attribute",
						MaxOpenFiles:             defaultMaxOpenFiles,
						DefaultSubPath:           defaultSubPath,
						AutoCreateDirectories:    true,
					},
				},
				unmarshaler: &pmetric.JSONUnmarshaler{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := tt.args.conf
			feI, err := newFileExporter(conf)
			assert.NoError(t, err)
			require.IsType(t, &groupingFileExporter{}, feI)
			gfe := feI.(*groupingFileExporter)

			testMetrics := func() pmetric.Metrics {
				td := testdata.GenerateMetricsTwoMetrics()
				testdata.GenerateMetricsOneCounterOneSummaryMetrics().ResourceMetrics().At(0).CopyTo(td.ResourceMetrics().AppendEmpty())
				testdata.GenerateMetricsOneMetricNoAttributes().ResourceMetrics().At(0).CopyTo(td.ResourceMetrics().AppendEmpty())
				td.ResourceMetrics().At(0).Resource().Attributes().PutStr("sub_path_attribute", "one")
				td.ResourceMetrics().At(1).Resource().Attributes().PutStr("sub_path_attribute", ".././two/two")
				return td
			}
			td := testMetrics()

			assert.NoError(t, gfe.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, gfe.consumeMetrics(context.Background(), td))
			assert.LessOrEqual(t, gfe.writers.Len(), conf.GroupByAttribute.MaxOpenFiles)

			assert.NoError(t, gfe.Shutdown(context.Background()))

			removeAttr := func(rMetrics pmetric.ResourceMetrics) pmetric.ResourceMetrics {
				if conf.GroupByAttribute.DeleteSubPathResourceAttribute {
					rMetrics.Resource().Attributes().Remove("sub_path_attribute")
				}
				return rMetrics
			}

			// the exporter may modify the test data, make sure we compare the results
			// to the original input
			td = testMetrics()
			pathResourceMetrics := map[string][]pmetric.ResourceMetrics{
				conf.Path + "/one":     {removeAttr(td.ResourceMetrics().At(0))},
				conf.Path + "/two/two": {removeAttr(td.ResourceMetrics().At(1))},
				conf.Path + "/MISSING": {removeAttr(td.ResourceMetrics().At(2))},
			}

			if conf.GroupByAttribute.DiscardIfAttributeNotFound {
				pathResourceMetrics[conf.Path+"/MISSING"] = []pmetric.ResourceMetrics{}
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
					got, err := tt.args.unmarshaler.UnmarshalMetrics(buf)
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

func TestGroupingFileExporterErrors(t *testing.T) {
	fe, err := newFileExporter(&Config{
		Path:       t.TempDir(),
		FormatType: formatTypeJSON,
		GroupByAttribute: &GroupByAttribute{
			SubPathResourceAttribute: "sub_path_attribute",
			DefaultSubPath:           defaultSubPath,
			MaxOpenFiles:             100,
			AutoCreateDirectories:    false,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	ld.ResourceLogs().At(0).Resource().Attributes().PutStr("sub_path_attribute", "one/two")

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
				Path:       b.TempDir(),
				FormatType: formatTypeJSON,
				GroupByAttribute: &GroupByAttribute{
					SubPathResourceAttribute: "sub_path_attribute",
					MaxOpenFiles:             100,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
		},
		{
			name: "grouping, 99 writers",
			conf: &Config{
				Path:       b.TempDir(),
				FormatType: formatTypeJSON,
				GroupByAttribute: &GroupByAttribute{
					SubPathResourceAttribute: "sub_path_attribute",
					MaxOpenFiles:             99,
					DefaultSubPath:           defaultSubPath,
					AutoCreateDirectories:    true,
				},
			},
		},
		{
			name: "grouping, 1 writer",
			conf: &Config{
				Path:       b.TempDir(),
				FormatType: formatTypeJSON,
				GroupByAttribute: &GroupByAttribute{
					SubPathResourceAttribute: "sub_path_attribute",
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
		td.ResourceSpans().At(0).Resource().Attributes().PutStr("sub_path_attribute", fmt.Sprintf("file%d", i))
		traces = append(traces, td)

		ld := testdata.GenerateLogsTwoLogRecordsSameResource()
		ld.ResourceLogs().At(0).Resource().Attributes().PutStr("sub_path_attribute", fmt.Sprintf("file%d", i))
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
