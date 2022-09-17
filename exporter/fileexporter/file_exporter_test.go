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

	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestFileTracesExporter(t *testing.T) {
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
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:        tempFileName(t),
					MarshalType: "proto",
				},
				unmarshaler: ptrace.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto: MarshalType is protobuf",
			args: args{
				conf: &Config{
					Path:        tempFileName(t),
					MarshalType: "protobuf",
				},
				unmarshaler: ptrace.NewProtoUnmarshaler(),
			},
		},
		{
			name: "proto: MarshalType is Protobuf",
			args: args{
				conf: &Config{
					Path:        tempFileName(t),
					MarshalType: "Protobuf",
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
			assert.NoError(t, err)
			defer fi.Close()
			br := bufio.NewReader(fi)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if fe.isJSON {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				assert.NoError(t, err)
				if isEnd {
					break
				}
				got, err := tt.args.unmarshaler.UnmarshalTraces(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, td, got)
			}
		})
	}
}

func TestFileTracesExporterError(t *testing.T) {
	mf := &errorWriter{}
	fe := &fileExporter{
		file:            mf,
		tracesMarshaler: errorMarshaler{},
	}
	require.NotNil(t, fe)

	td := testdata.GenerateTracesTwoSpansSameResource()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeTraces(context.Background(), td))
	assert.NoError(t, fe.Shutdown(context.Background()))
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
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:        tempFileName(t),
					MarshalType: "protobuf",
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
			assert.NoError(t, err)
			defer fi.Close()
			br := bufio.NewReader(fi)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if fe.isJSON {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				assert.NoError(t, err)
				if isEnd {
					break
				}
				got, err := tt.args.unmarshaler.UnmarshalMetrics(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, md, got)
			}
		})
	}

}

func TestFileMetricsExporterError(t *testing.T) {
	mf := &errorWriter{}
	fe := &fileExporter{
		file:             mf,
		metricsMarshaler: errorMarshaler{},
	}
	require.NotNil(t, fe)

	md := testdata.GenerateMetricsTwoMetrics()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeMetrics(context.Background(), md))
	assert.NoError(t, fe.Shutdown(context.Background()))
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
			name: "Proto: default configuration",
			args: args{
				conf: &Config{
					Path:        tempFileName(t),
					MarshalType: "Protobuf",
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
			assert.NoError(t, err)
			defer fi.Close()
			br := bufio.NewReader(fi)
			for {
				buf, isEnd, err := func() ([]byte, bool, error) {
					if fe.isJSON {
						return readJSONMessage(br)
					}
					return readMessageFromStream(br)
				}()
				assert.NoError(t, err)
				if isEnd {
					break
				}
				got, err := tt.args.unmarshaler.UnmarshalLogs(buf)
				assert.NoError(t, err)
				assert.EqualValues(t, ld, got)
			}
		})
	}
}

func TestFileLogsExporterErrors(t *testing.T) {
	mf := &errorWriter{}
	fe := &fileExporter{
		file:          mf,
		logsMarshaler: errorMarshaler{},
	}
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeLogs(context.Background(), ld))
	assert.NoError(t, fe.Shutdown(context.Background()))
}

func Test_fileExporter_Capabilities(t *testing.T) {
	fe := newFileExporter(
		&Config{
			Path:     tempFileName(t),
			Rotation: Rotation{MaxMegabytes: 1},
		})
	require.NotNil(t, fe)
	require.NotNil(t, fe.Capabilities())
}

func TestExportMessageAsBuffer(t *testing.T) {
	fe := newFileExporter(&Config{
		Path: tempFileName(t),
		Rotation: Rotation{
			MaxMegabytes: 1,
		},
		MarshalType: "proto",
	})
	require.NotNil(t, fe)
	//
	ld := testdata.GenerateLogsManyLogRecordsSameResource(15000)
	marshaler := plog.NewProtoMarshaler()
	buf, err := marshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	assert.Error(t, exportMessageAsBuffer(fe, buf))
	assert.NoError(t, fe.Shutdown(context.Background()))

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

// errorWriter is an io.Writer that will return an error all ways
type errorWriter struct {
}

func (e errorWriter) Write([]byte) (n int, err error) {
	return 0, errors.New("all ways return error")
}

func (e *errorWriter) Close() error {
	return nil
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
