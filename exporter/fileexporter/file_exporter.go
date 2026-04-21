// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// errFileExporterShutdown is returned when a consume* call arrives after
// Shutdown has already run. Previously Shutdown nil'd out e.writer and a
// racing consume* call panicked with nil pointer dereference (#46871).
var errFileExporterShutdown = errors.New("fileexporter: already shut down")

// fileExporter is the implementation of file exporter that writes telemetry data to a file
type fileExporter struct {
	conf       *Config
	marshaller *marshaller
	// writerMu serializes access to writer between Shutdown and the
	// consume* callers so a racing consume* call cannot observe writer
	// after Shutdown has freed it (#46871).
	writerMu sync.RWMutex
	writer   *fileWriter
}

// loadWriter returns the current writer under writerMu; the returned
// pointer is safe to use for the duration of the call that captured it.
func (e *fileExporter) loadWriter() *fileWriter {
	e.writerMu.RLock()
	defer e.writerMu.RUnlock()
	return e.writer
}

func (e *fileExporter) consumeTraces(_ context.Context, td ptrace.Traces) error {
	w := e.loadWriter()
	if w == nil {
		return errFileExporterShutdown
	}
	buf, err := e.marshaller.marshalTraces(td)
	if err != nil {
		return err
	}
	return w.export(buf)
}

func (e *fileExporter) consumeMetrics(_ context.Context, md pmetric.Metrics) error {
	w := e.loadWriter()
	if w == nil {
		return errFileExporterShutdown
	}
	buf, err := e.marshaller.marshalMetrics(md)
	if err != nil {
		return err
	}
	return w.export(buf)
}

func (e *fileExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	w := e.loadWriter()
	if w == nil {
		return errFileExporterShutdown
	}
	buf, err := e.marshaller.marshalLogs(ld)
	if err != nil {
		return err
	}
	return w.export(buf)
}

func (e *fileExporter) consumeProfiles(_ context.Context, pd pprofile.Profiles) error {
	w := e.loadWriter()
	if w == nil {
		return errFileExporterShutdown
	}
	buf, err := e.marshaller.marshalProfiles(pd)
	if err != nil {
		return err
	}
	return w.export(buf)
}

// Start starts the flush timer if set.
func (e *fileExporter) Start(_ context.Context, host component.Host) error {
	var err error
	e.marshaller, err = newMarshaller(e.conf, host)
	if err != nil {
		return err
	}
	export := buildExportFunc(e.conf)

	// Optionally ensure the output directory exists.
	if e.conf.CreateDirectory {
		dir := filepath.Dir(e.conf.Path)
		perm := os.FileMode(0o755)
		if e.conf.directoryPermissionsParsed != 0 {
			perm = os.FileMode(e.conf.directoryPermissionsParsed)
		}
		err = os.MkdirAll(dir, perm)
		if err != nil {
			return err
		}
	}

	w, err := newFileWriter(e.conf.Path, e.conf.Append, e.conf.Rotation, e.conf.FlushInterval, export)
	if err != nil {
		return err
	}
	w.start()
	e.writerMu.Lock()
	e.writer = w
	e.writerMu.Unlock()
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops the flush ticker if set.
func (e *fileExporter) Shutdown(context.Context) error {
	e.writerMu.Lock()
	w := e.writer
	e.writer = nil
	e.writerMu.Unlock()
	if w == nil {
		return nil
	}
	return w.shutdown()
}
