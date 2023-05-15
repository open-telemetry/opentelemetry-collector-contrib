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

// exportFunc defines how to export encoded telemetry data.
type exportFunc func(e *fileExporter, buf []byte) error

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
	exporter   exportFunc

	flushInterval time.Duration
	flushTicker   *time.Ticker
	stopTicker    chan struct{}
}

type binaryExporter struct {
	*fileExporter
}

type lineExporter struct {
	*fileExporter
}

func (e *fileExporter) consumeTraces(_ context.Context, td ptrace.Traces) error {
	buf, err := e.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	buf = e.compressor(buf)
	return e.exporter(e, buf)
}

func (e *fileExporter) consumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := e.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	buf = e.compressor(buf)
	return e.exporter(e, buf)
}

func (e fileExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := e.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	buf = e.compressor(buf)
	return e.exporter(e, buf)
}

func (e lineExporter) Write(buf []byte) (int, error) {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	n1, err := e.file.Write(buf)
	if err != nil {
		return err
	}
	n2, err := io.WriteString(e.file, "\n")
	if err != nil {
		return err
	}
	return n1 + n2, nil
}

func (e *binaryExporter) Write(buf []byte) (int, error) {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	// write the size of each message before writing the message itself.  https://developers.google.com/protocol-buffers/docs/techniques
	// each encoded object is preceded by 4 bytes (an unsigned 32 bit integer)
	data := make([]byte, 4, 4+len(buf))
	binary.BigEndian.PutUint32(data, uint32(len(buf)))
	data = append(data, buf...)
	if err := binary.Write(e.file, binary.BigEndian, data); err != nil {
		return -1, err
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

func (e *fileExporter) buildExportFunc(cfg *Config) (io.Writer, error) {
	if cfg.FormatType == formatTypeProto {
		return binaryExporter{e}
	}
	// if the data format is JSON and needs to be compressed, telemetry data can't be written to file in JSON format.
	if cfg.FormatType == formatTypeJSON && cfg.Compression != "" {
		return binaryExporter{fileExporter: e}, nil
	}
	return lineExporter{fileExporter: e}, nil
}
