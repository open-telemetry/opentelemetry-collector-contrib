// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"go.uber.org/zap"
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
	path     string
	file     io.WriteCloser
	mutex    sync.Mutex
	logger   *zap.Logger

	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler

	compression string
	compressor  compressFunc

	formatType string
	exporter   io.WriteCloser

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

func NewLineWriter(cfg *Config, logger *zap.Logger, file io.WriteCloser) io.WriteCloser {
	if cfg.Compression == "zstd" {
		if fw, err := zstd.NewWriter(file); err == nil {
			// flushing the compressed writer every second.
			go func() {
				for {
					time.Sleep(1 * time.Second)
					if fw.Flush() != nil {
						return
					}
				}
			}()

			file = fw
		} else {
			logger.Debug("Unable to create compressed writer", zap.Error(err))
		}
	}

	return &lineWriter{
		file: file,
	}
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

func (lw *lineWriter) Close() error {
	return lw.file.Close()
}

type fileWriter struct {
	mutex sync.Mutex
	file  io.WriteCloser
}

func NewFileWriter(cfg *Config, logger *zap.Logger, file io.WriteCloser) io.WriteCloser {
	if cfg.Compression == "zstd" {
		if fw, err := zstd.NewWriter(file); err == nil {
			// flushing the compressed writer every second.
			go func() {
				for {
					time.Sleep(1 * time.Second)
					if fw.Flush() != nil {
						return
					}
				}
			}()
			file = fw
		} else {
			logger.Debug("Unable to create compressed writer", zap.Error(err))
		}
	}

	return &fileWriter{
		file: file,
	}
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

func (fw *fileWriter) Close() error {
	return fw.file.Close()
}

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
	return e.exporter.Close()
}

func (e *fileExporter) createExporterWriter(cfg *Config) io.WriteCloser {
	if cfg.FormatType == formatTypeProto {
		return NewFileWriter(cfg, e.logger, e.file)
	}
	return NewLineWriter(cfg, e.logger, e.file)
}
