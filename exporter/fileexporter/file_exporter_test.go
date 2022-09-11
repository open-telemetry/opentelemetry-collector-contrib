// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package fileexporter

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/gozstd"
	"gopkg.in/natefinch/lumberjack.v2"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFileTracesExporter_JSONMarshal(t *testing.T) {
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
					Path: tempFileName(t),
				},
				unmarshaler: ptrace.NewJSONUnmarshaler(),
			},
		},
		{
			name: "json: Zstd compression option",
			args: args{
				conf: &Config{
					Path:       tempFileName(t),
					ZstdOption: true,
				},
				unmarshaler: ptrace.NewJSONUnmarshaler(),
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
				},
				unmarshaler: ptrace.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto: Zstd compression option",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					ZstdOption:      true,
				},
				unmarshaler: ptrace.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto:  an option to self-rotate log files",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					RollingLoggerOptions: RollingLoggerOptions{
						MaxSize:    10,
						MaxAge:     1,
						MaxBackups: 3,
						LocalTime:  false,
					},
				},
				unmarshaler: ptrace.NewProtoUnmarshaler(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)

			td := testdata.GenerateTracesTwoSpansSameResource()
			assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
			assert.NoError(t, fe.ConsumeTraces(context.Background(), td))
			assert.NoError(t, fe.ConsumeTraces(context.Background(), td))
			assert.NoError(t, fe.Shutdown(context.Background()))

			fi, err := os.Open(fe.path)
			defer fi.Close()
			assert.NoError(t, err)
			br := bufio.NewReader(fi)
			isJson := !(tt.args.conf.ZstdOption || tt.args.conf.PbMarshalOption)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if isJson {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				if isEnd {
					break
				}
				if tt.args.conf.ZstdOption {
					buf, err = gozstd.Decompress(nil, buf)
					assert.NoError(t, err)
				}
				got, err := tt.args.unmarshaler.UnmarshalTraces(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, td, got)
			}
		})
	}
}

func TestFileTracesExporterError(t *testing.T) {
	type args struct {
		conf *Config
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "json",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
		{
			name: "proto",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					PbMarshalOption:      true,
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)
			// If the length of the write is greater than MaxSize, an error is returned.
			td := testdata.GenerateTracesManySpansSameResource(10000)
			assert.Error(t, fe.ConsumeTraces(context.Background(), td))
			assert.NoError(t, fe.Shutdown(context.Background()))
		})
	}

}

func TestFileMetricsExporter(t *testing.T) {
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
					Path: tempFileName(t),
				},
				unmarshaler: pmetric.NewJSONUnmarshaler(),
			},
		},
		{
			name: "json: Zstd compression option",
			args: args{
				conf: &Config{
					Path:       tempFileName(t),
					ZstdOption: true,
				},
				unmarshaler: pmetric.NewJSONUnmarshaler(),
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
				},
				unmarshaler: pmetric.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto: Zstd compression option",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					ZstdOption:      true,
				},
				unmarshaler: pmetric.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto:  an option to self-rotate log files",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					RollingLoggerOptions: RollingLoggerOptions{
						MaxSize:    10,
						MaxAge:     1,
						MaxBackups: 3,
						LocalTime:  false,
					},
				},
				unmarshaler: pmetric.NewProtoUnmarshaler(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)

			md := testdata.GenerateMetricsTwoMetrics()
			assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
			assert.NoError(t, fe.ConsumeMetrics(context.Background(), md))
			assert.NoError(t, fe.ConsumeMetrics(context.Background(), md))
			assert.NoError(t, fe.Shutdown(context.Background()))

			fi, err := os.Open(fe.path)
			defer fi.Close()
			assert.NoError(t, err)
			br := bufio.NewReader(fi)
			isJson := !(tt.args.conf.ZstdOption || tt.args.conf.PbMarshalOption)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if isJson {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				if isEnd {
					break
				}
				if tt.args.conf.ZstdOption {
					buf, err = gozstd.Decompress(nil, buf)
					assert.NoError(t, err)
				}
				got, err := tt.args.unmarshaler.UnmarshalMetrics(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, md, got)
			}
		})
	}

}

func TestFileMetricsExporterError(t *testing.T) {
	type args struct {
		conf *Config
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "json",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
		{
			name: "proto",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					PbMarshalOption:      true,
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)
			// If the length of the write is greater than MaxSize, an error is returned.
			md := testdata.GenerateMetricsManyMetricsSameResource(10000)
			// Cannot call Start since we inject directly the WriterCloser.
			assert.Error(t, fe.ConsumeMetrics(context.Background(), md))
			assert.NoError(t, fe.Shutdown(context.Background()))
		})
	}
}

func TestFileLogsExporter(t *testing.T) {
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
					Path: tempFileName(t),
				},
				unmarshaler: plog.NewJSONUnmarshaler(),
			},
		},
		{
			name: "json: Zstd compression option",
			args: args{
				conf: &Config{
					Path:       tempFileName(t),
					ZstdOption: true,
				},
				unmarshaler: plog.NewJSONUnmarshaler(),
			},
		},
		{
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
				},
				unmarshaler: plog.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto: Zstd compression option",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					ZstdOption:      true,
				},
				unmarshaler: plog.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto:  an option to self-rotate log files",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
					RollingLoggerOptions: RollingLoggerOptions{
						MaxSize:    10,
						MaxAge:     1,
						MaxBackups: 3,
						LocalTime:  false,
					},
				},
				unmarshaler: plog.NewProtoUnmarshaler(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)

			ld := testdata.GenerateLogsTwoLogRecordsSameResource()
			assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
			assert.NoError(t, fe.ConsumeLogs(context.Background(), ld))
			assert.NoError(t, fe.ConsumeLogs(context.Background(), ld))
			assert.NoError(t, fe.Shutdown(context.Background()))

			fi, err := os.Open(fe.path)
			defer fi.Close()
			assert.NoError(t, err)
			br := bufio.NewReader(fi)
			isJson := !(tt.args.conf.ZstdOption || tt.args.conf.PbMarshalOption)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if isJson {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				if isEnd {
					break
				}
				if tt.args.conf.ZstdOption {
					buf, err = gozstd.Decompress(nil, buf)
					assert.NoError(t, err)
				}
				got, err := tt.args.unmarshaler.UnmarshalLogs(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, ld, got)
			}
		})
	}
}

func TestFileLogsExporterErrors(t *testing.T) {
	type args struct {
		conf *Config
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "json",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
		{
			name: "proto",
			args: args{
				conf: &Config{
					Path:                 tempFileName(t),
					PbMarshalOption:      true,
					RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := newFileExporter(tt.args.conf)
			require.NotNil(t, fe)
			// If the length of the write is greater than MaxSize, an error is returned.
			ld := testdata.GenerateLogsManyLogRecordsSameResource(15000)
			// Cannot call Start since we inject directly the WriterCloser.
			assert.Error(t, fe.ConsumeLogs(context.Background(), ld))
			assert.NoError(t, fe.Shutdown(context.Background()))
		})
	}

}

func Test_fileExporter_Capabilities(t *testing.T) {
	fe := newFileExporter(
		&Config{
			Path:                 tempFileName(t),
			RollingLoggerOptions: RollingLoggerOptions{MaxSize: 1},
		})
	require.NotNil(t, fe)
	require.NotNil(t, fe.Capabilities())
}

// tempFileName provides a temporary file name for testing.
func tempFileName(t *testing.T) string {
	tmpfile, err := os.CreateTemp("", "*")
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())
	socket := tmpfile.Name()
	require.NoError(t, os.Remove(socket))
	return socket
}

func readMessageFromStream(br *bufio.Reader) ([]byte, bool, error) {
	var buf []byte
	line, _, c := br.ReadLine()
	if c == io.EOF {
		return nil, true, nil
	}
	size := cast.ToInt(string(line))
	buf = make([]byte, size)
	if _, err := br.Read(buf); err != nil {
		return nil, false, err
	}
	return buf, false, nil
}

func readJSONMessage(br *bufio.Reader) ([]byte, bool, error) {
	buf, _, c := br.ReadLine()
	if c == io.EOF {
		return nil, true, nil
	}
	return buf, false, nil
}

// errorMarshaler is an Marshaler that will return an error all ways
type errorMarshaler struct {
}

func (m errorMarshaler) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	return nil, errors.New("all ways return error")
}

func (m errorMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return nil, errors.New("all ways return error")
}

func (m errorMarshaler) MarshalLogs(md plog.Logs) ([]byte, error) {
	return nil, errors.New("all ways return error")
}

func TestConsumeTracesMarshalError(t *testing.T) {
	file := tempFileName(t)
	e := &fileExporter{
		path:            file,
		tracesMarshaler: errorMarshaler{},
		logger: &lumberjack.Logger{
			Filename: file,
		},
	}
	td := testdata.GenerateTracesTwoSpansSameResource()
	assert.Error(t, e.ConsumeTraces(context.Background(), td))
}

func TestConsumeMetricsMarshalError(t *testing.T) {
	file := tempFileName(t)
	e := &fileExporter{
		path:             file,
		metricsMarshaler: errorMarshaler{},
		logger: &lumberjack.Logger{
			Filename: file,
		},
	}
	md := testdata.GenerateMetricsTwoMetrics()
	assert.Error(t, e.ConsumeMetrics(context.Background(), md))
}

func TestConsumeLogsMarshalError(t *testing.T) {
	file := tempFileName(t)
	e := &fileExporter{
		path:          file,
		logsMarshaler: errorMarshaler{},
		logger: &lumberjack.Logger{
			Filename: file,
		},
	}
	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	assert.Error(t, e.ConsumeLogs(context.Background(), ld))
}
