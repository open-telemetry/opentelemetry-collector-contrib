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
	"io"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Marshaler configuration used for marhsaling Protobuf
var tracesMarshalers = map[string]ptrace.Marshaler{
	formatTypeJSON:  &ptrace.JSONMarshaler{},
	formatTypeProto: &ptrace.ProtoMarshaler{},
}
var metricsMarshalers = map[string]pmetric.Marshaler{
	formatTypeJSON:  &pmetric.JSONMarshaler{},
	formatTypeProto: &pmetric.ProtoMarshaler{},
}
var logsMarshalers = map[string]plog.Marshaler{
	formatTypeJSON:  &plog.JSONMarshaler{},
	formatTypeProto: &plog.ProtoMarshaler{},
}

// fileExporter is the implementation of file exporter that writes telemetry data to a file
type fileExporter struct {
	path  string
	file  io.WriteCloser
	mutex sync.Mutex

	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler

	compression string
	compressor  compressFunc

	formatType string
	exporter   io.Writer

	flushInterval time.Duration
	flushTicker   *time.Ticker
	stopTicker    chan struct{}
}

func (e *fileExporter) consumeTraces(_ context.Context, td ptrace.Traces) error {
	buf, err := e.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	_, err = e.exporter.Write(buf)
	return err
}

func (e *fileExporter) consumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := e.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	_, err = e.exporter.Write(buf)
	return err
}

func (e *fileExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := e.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	_, err = e.exporter.Write(buf)
	return err
}

type lineWriter struct {
	mutex sync.Mutex
	file  io.WriteCloser
}

func (lw *lineWriter) Write(buf []byte) (int, error) {
	// Ensure only one write operation happens at a time.
	lw.mutex.Lock()
	defer lw.mutex.Unlock()
	if _, err := lw.file.Write(buf); err != nil {
		return 0, err
	}
	if _, err := io.WriteString(lw.file, "\n"); err != nil {
		return 0, err
	}
	return 1 + len(buf), nil
}

type fileWriter struct {
	mutex sync.Mutex
	file  io.WriteCloser
}

func (fw *fileWriter) Write(buf []byte) (int, error) {
	// Ensure only one write operation happens at a time.
	fw.mutex.Lock()
	defer fw.mutex.Unlock()
	// write the size of each message before writing the message itself.  https://developers.google.com/protocol-buffers/docs/techniques
	// each encoded object is preceded by 4 bytes (an unsigned 32 bit integer)
	data := make([]byte, 4, 4+len(buf))
	binary.BigEndian.PutUint32(data, uint32(len(buf)))
	data = append(data, buf...)
	if err := binary.Write(fw.file, binary.BigEndian, data); err != nil {
		return 0, err
	}

	return len(data), nil
}

// startFlusher starts the flusher.
// It does not check the flushInterval
func (e *fileExporter) startFlusher() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	ff, ok := e.file.(interface{ flush() error })
	if !ok {
		// Just in case.
		return
	}

	// Create the stop channel.
	e.stopTicker = make(chan struct{})
	// Start the ticker.
	e.flushTicker = time.NewTicker(e.flushInterval)
	go func() {
		for {
			select {
			case <-e.flushTicker.C:
				e.mutex.Lock()
				ff.flush()
				e.mutex.Unlock()
			case <-e.stopTicker:
				return
			}
		}
	}()
}

// Start starts the flush timer if set.
func (e *fileExporter) Start(context.Context, component.Host) error {
	if e.flushInterval > 0 {
		e.startFlusher()
	}
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops the flush ticker if set.
func (e *fileExporter) Shutdown(context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// Stop the flush ticker.
	if e.flushTicker != nil {
		e.flushTicker.Stop()
		// Stop the go routine.
		close(e.stopTicker)
	}
	return e.file.Close()
}

func (e *fileExporter) createExporterWriter(cfg *Config) io.Writer {
	if cfg.FormatType == formatTypeProto {
		if cfg.Compression == "zstd" {
			if fw, err := zstd.NewWriter(e.file); err == nil {
				return &fileWriter{
					file: fw,
				}
			}
		}

		return &fileWriter{
			file: e.file,
		}
	}

	// if the data format is JSON and needs to be compressed, telemetry data can't be written to file in JSON format.
	if cfg.FormatType == formatTypeJSON {
		if cfg.Compression == "zstd" {
			if fw, err := zstd.NewWriter(e.file); err == nil {
				return &lineWriter{
					file: fw,
				}
			}
		}
	}

	return &lineWriter{
		file: e.file,
	}
}
