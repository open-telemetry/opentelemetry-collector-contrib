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
	"io"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/spf13/cast"
	"github.com/valyala/gozstd"
	"gopkg.in/natefinch/lumberjack.v2"
)

// fileExporter is the implementation of file exporter that writes telemetry data to a file
// in Protobuf-JSON format.
type fileExporter struct {
	path  string
	mutex sync.Mutex

	isCompressed   bool
	isProtoMarshal bool

	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler

	logger *lumberjack.Logger
}

func newFileExporter(conf *Config) *fileExporter {
	tracesMarshaler, metricsMarshaler, logMarshaler := func() (ptrace.Marshaler, pmetric.Marshaler, plog.Marshaler) {
		if conf.PbMarshalOption {
			return ptrace.NewProtoMarshaler(), pmetric.NewProtoMarshaler(), plog.NewProtoMarshaler()
		}
		return ptrace.NewJSONMarshaler(), pmetric.NewJSONMarshaler(), plog.NewJSONMarshaler()
	}()
	return &fileExporter{
		path:             conf.Path,
		isCompressed:     conf.ZstdOption,
		isProtoMarshal:   conf.PbMarshalOption,
		tracesMarshaler:  tracesMarshaler,
		metricsMarshaler: metricsMarshaler,
		logsMarshaler:    logMarshaler,
		logger: &lumberjack.Logger{
			Filename:   conf.Path,
			MaxSize:    conf.RollingLoggerOptions.MaxSize,
			MaxAge:     conf.RollingLoggerOptions.MaxAge,
			MaxBackups: conf.RollingLoggerOptions.MaxBackups,
			LocalTime:  conf.RollingLoggerOptions.LocalTime,
		},
	}
}

func (e *fileExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *fileExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	buf, err := e.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	if e.isCompressed {
		buf = gozstd.Compress(nil, buf)
	}
	isJSON := !(e.isCompressed || e.isProtoMarshal)
	return exportMessage(e, buf, isJSON)
}

func (e *fileExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := e.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	if e.isCompressed {
		buf = gozstd.Compress(nil, buf)
	}
	isJSON := !(e.isCompressed || e.isProtoMarshal)
	return exportMessage(e, buf, isJSON)
}

func (e *fileExporter) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := e.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	if e.isCompressed {
		buf = gozstd.Compress(nil, buf)
	}
	isJSON := !(e.isCompressed || e.isProtoMarshal)
	return exportMessage(e, buf, isJSON)
}

func exportMessage(e *fileExporter, buf []byte, isJSON bool) error {
	if isJSON {
		return exportMessageAsLine(e, buf)
	}
	return exportStreamMessages(e, buf)
}

func exportMessageAsLine(e *fileExporter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, err := e.logger.Write(buf); err != nil {
		return err
	}
	if _, err := io.WriteString(e.logger, "\n"); err != nil {
		return err
	}
	return nil
}

func exportStreamMessages(e *fileExporter, buf []byte) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	sizeLine := cast.ToString(len(buf)) + "\n"
	if _, err := e.logger.Write([]byte(sizeLine)); err != nil {
		return err
	}
	if _, err := e.logger.Write(buf); err != nil {
		return err
	}
	return nil
}

func (e *fileExporter) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
func (e *fileExporter) Shutdown(context.Context) error {
	return e.logger.Close()
}
