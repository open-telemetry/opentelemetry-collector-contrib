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
	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"io"
	"os"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/natefinch/lumberjack.v2"
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
			name: "default configuration",
			args: args{
				conf: &Config{
					Path: tempFileName(t),
				},
				unmarshaler: ptrace.NewJSONUnmarshaler(),
			},
		},
		{
			name: "encode data using a protobuf stream ",
			args: args{
				conf: &Config{
					Path:            tempFileName(t),
					PbMarshalOption: true,
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
			assert.NoError(t, fe.Shutdown(context.Background()))

			fi, err := os.Open(fe.path)
			defer fi.Close()
			assert.NoError(t, err)
			br := bufio.NewReader(fi)
			for {
				var buf []byte
				line, _, c := br.ReadLine()
				if c == io.EOF {
					break
				}
				if tt.args.conf.PbMarshalOption {
					size := cast.ToInt(string(line))

					buf = make([]byte, size)
					_, err = br.Read(buf)
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
	fe := &fileExporter{
		logger: &lumberjack.Logger{
			Filename: tempFileName(t),
		}}
	require.NotNil(t, fe)

	td := testdata.GenerateTracesTwoSpansSameResource()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeTraces(context.Background(), td))
	assert.NoError(t, fe.Shutdown(context.Background()))
}

func TestFileMetricsExporter(t *testing.T) {
	fe := &fileExporter{path: tempFileName(t)}
	require.NotNil(t, fe)

	md := testdata.GenerateMetricsTwoMetrics()
	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, fe.ConsumeMetrics(context.Background(), md))
	assert.NoError(t, fe.Shutdown(context.Background()))

	unmarshaler := pmetric.NewJSONUnmarshaler()
	buf, err := os.ReadFile(fe.path)
	assert.NoError(t, err)
	got, err := unmarshaler.UnmarshalMetrics(buf)
	assert.NoError(t, err)
	assert.EqualValues(t, md, got)
}

func TestFileMetricsExporterError(t *testing.T) {
	fe := &fileExporter{logger: &lumberjack.Logger{
		Filename: tempFileName(t),
	}}
	require.NotNil(t, fe)

	md := testdata.GenerateMetricsTwoMetrics()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeMetrics(context.Background(), md))
	assert.NoError(t, fe.Shutdown(context.Background()))
}

func TestFileLogsExporter(t *testing.T) {
	fe := &fileExporter{path: tempFileName(t)}
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, fe.ConsumeLogs(context.Background(), ld))
	assert.NoError(t, fe.Shutdown(context.Background()))

	unmarshaler := plog.NewJSONUnmarshaler()
	buf, err := os.ReadFile(fe.path)
	assert.NoError(t, err)
	got, err := unmarshaler.UnmarshalLogs(buf)
	assert.NoError(t, err)
	assert.EqualValues(t, ld, got)
}

func TestFileLogsExporterErrors(t *testing.T) {
	fe := &fileExporter{logger: &lumberjack.Logger{
		Filename: tempFileName(t),
	}}
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	// Cannot call Start since we inject directly the WriterCloser.
	assert.Error(t, fe.ConsumeLogs(context.Background(), ld))
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
