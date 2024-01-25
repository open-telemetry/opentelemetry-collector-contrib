// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package fileexporter

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

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

// TODO: write the above test for logs and traces as well

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

// func configWithDefaults(t testing.TB, data map[string]any) *Config {
// 	cfg := createDefaultConfig().(*Config)
// 	err := cfg.Unmarshal(confmap.NewFromStringMap(data))
// 	require.NoError(t, err)

// 	return cfg
// }

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
		require.NoError(b, fe.Start(context.Background(), componenttest.NewNopHost()))

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				fe.consumeTraces(ctx, traces[i%len(traces)])
				fe.consumeLogs(ctx, logs[i%len(logs)])
			}
		})

		assert.NoError(b, fe.Shutdown(context.Background()))
	}
}
