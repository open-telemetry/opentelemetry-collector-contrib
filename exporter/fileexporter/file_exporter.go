// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"encoding/binary"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	formatTypeProto = "proto"
	formatTypeJSON  = "json"
)

// Marshaler configuration used for marhsaling Protobuf
var tracesMarshalers = map[string]ptrace.Marshaler{
	formatTypeJSON:  ptrace.NewJSONMarshaler(),
	formatTypeProto: ptrace.NewProtoMarshaler(),
}
var metricsMarshalers = map[string]pmetric.Marshaler{
	formatTypeJSON:  pmetric.NewJSONMarshaler(),
	formatTypeProto: pmetric.NewProtoMarshaler(),
}
var logsMarshalers = map[string]plog.Marshaler{
	formatTypeJSON:  plog.NewJSONMarshaler(),
	formatTypeProto: plog.NewProtoMarshaler(),
}

// fileExporter is the implementation of file exporter that writes telemetry data to a file
type fileExporter struct {
	path  string
	file  io.WriteCloser
	mutex sync.Mutex

	formatType string
	//
	exportFunc func(e *fileExporter, buf []byte) error
}

func newFileExporter(cfg *Config) *fileExporter {
	format := func() string {
		if cfg.FormatType == "" {
			return formatTypeJSON
		}
		return cfg.FormatType
	}()
	return &fileExporter{
		path:       cfg.Path,
		formatType: format,
		file: &lumberjack.Logger{
			Filename:   cfg.Path,
			MaxSize:    cfg.Rotation.MaxMegabytes,
			MaxAge:     cfg.Rotation.MaxDays,
			MaxBackups: cfg.Rotation.MaxBackups,
			LocalTime:  cfg.Rotation.LocalTime,
		},
		exportFunc: buildExportFunc(cfg),
	}
}
func (e *fileExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *fileExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	buf, err := tracesMarshalers[e.formatType].MarshalTraces(td)
	if err != nil {
		return err
	}
	return e.exportFunc(e, buf)
}

func (e *fileExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := metricsMarshalers[e.formatType].MarshalMetrics(md)
	if err != nil {
		return err
	}
	return e.exportFunc(e, buf)
}

func (e *fileExporter) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := logsMarshalers[e.formatType].MarshalLogs(ld)
	if err != nil {
		return err
	}
	return e.exportFunc(e, buf)
}

func exportMessageAsLine(e *fileExporter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, err := e.file.Write(buf); err != nil {
		return err
	}
	if _, err := io.WriteString(e.file, "\n"); err != nil {
		return err
	}
	return nil
}

func exportMessageAsBuffer(e *fileExporter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// write the size of each message before writing the message itself.  https://developers.google.com/protocol-buffers/docs/techniques
	if err := binary.Write(e.file, binary.BigEndian, int32(len(buf))); err != nil {
		return err
	}
	if err := binary.Write(e.file, binary.BigEndian, buf); err != nil {
		return err
	}
	return nil
}

func (e *fileExporter) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
func (e *fileExporter) Shutdown(context.Context) error {
	return e.file.Close()
}

func buildExportFunc(cfg *Config) func(e *fileExporter, buf []byte) error {
	if cfg.FormatType == formatTypeProto {
		return exportMessageAsBuffer
	}
	return exportMessageAsLine
}
