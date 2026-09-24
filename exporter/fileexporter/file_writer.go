// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
)

// exportFunc defines how to export encoded telemetry data.
type exportFunc func(e *fileWriter, buf []byte) error

type fileWriter struct {
	path  string
	file  io.WriteCloser
	mutex sync.Mutex

	exporter exportFunc

	flushInterval time.Duration
	flushTicker   *time.Ticker
	stopTicker    chan struct{}
}

func exportMessageAsLine(w *fileWriter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if _, err := w.file.Write(buf); err != nil {
		return err
	}
	if _, err := io.WriteString(w.file, "\n"); err != nil {
		return err
	}
	return nil
}

func exportMessageAsBuffer(w *fileWriter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	w.mutex.Lock()
	defer w.mutex.Unlock()
	// write the size of each message before writing the message itself.  https://developers.google.com/protocol-buffers/docs/techniques
	// each encoded object is preceded by 4 bytes (an unsigned 32 bit integer)
	data := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(buf)), uint32(len(buf)))
	_, err := w.file.Write(append(data, buf...))
	return err
}

func (w *fileWriter) export(buf []byte) error {
	return w.exporter(w, buf)
}

// startFlusher starts the flusher.
// It does not check the flushInterval
func (w *fileWriter) startFlusher() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	ff, ok := w.file.(interface{ flush() error })
	if !ok {
		// Just in case.
		return
	}

	// Create the stop channel.
	w.stopTicker = make(chan struct{})
	// Start the ticker.
	w.flushTicker = time.NewTicker(w.flushInterval)
	go func() {
		for {
			select {
			case <-w.flushTicker.C:
				w.mutex.Lock()
				ff.flush()
				w.mutex.Unlock()
			case <-w.stopTicker:
				w.flushTicker.Stop()
				w.flushTicker = nil
				return
			}
		}
	}()
}

// Start starts the flush timer if set.
func (w *fileWriter) start() {
	if w.flushInterval > 0 {
		w.startFlusher()
	}
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops the flush ticker if set.
func (w *fileWriter) shutdown() error {
	// Stop the flush ticker.
	if w.flushTicker != nil {
		// Stop the go routine.
		w.mutex.Lock()
		close(w.stopTicker)
		w.mutex.Unlock()
	}
	return w.file.Close()
}

// buildExportFunc selects the framing used to write encoded telemetry.
func buildExportFunc(cfg *Config) exportFunc {
	// Per-message compression (gate off) emits binary, so JSON lines need the file itself uncompressed.
	// Native compression frames by `format`, the same as uncompressed output. See #49328.
	if cfg.FormatType == formatTypeJSON && (cfg.Compression == "" || metadata.ExporterFileNativeCompressionFeatureGate.IsEnabled()) {
		return exportMessageAsLine
	}
	return exportMessageAsBuffer
}
