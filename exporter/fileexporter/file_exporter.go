// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func (e *fileExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := e.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	buf = e.compressor(buf)
	return e.exporter(e, buf)
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
	// each encoded object is preceded by 4 bytes (an unsigned 32 bit integer)
	data := make([]byte, 4, 4+len(buf))
	binary.BigEndian.PutUint32(data, uint32(len(buf)))

	return binary.Write(e.file, binary.BigEndian, append(data, buf...))
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

func buildExportFunc(cfg *Config) func(e *fileExporter, buf []byte) error {
	if cfg.FormatType == formatTypeProto {
		return exportMessageAsBuffer
	}
	// if the data format is JSON and needs to be compressed, telemetry data can't be written to file in JSON format.
	if cfg.FormatType == formatTypeJSON && cfg.Compression != "" {
		return exportMessageAsBuffer
	}
	return exportMessageAsLine
}
