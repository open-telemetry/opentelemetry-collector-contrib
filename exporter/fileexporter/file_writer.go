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
	// Protected by mutex
	refs    int  // number of active references to this writer
	evicted bool // true if the writer has been evicted from the LRU
	closed  bool
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
	data := make([]byte, 4, 4+len(buf))
	binary.BigEndian.PutUint32(data, uint32(len(buf)))

	return binary.Write(w.file, binary.BigEndian, append(data, buf...))
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
	// use the local copies so the goroutine does not read these fields without the lock.
	ticker := w.flushTicker
	stop := w.stopTicker
	go func() {
		for {
			select {
			case <-ticker.C:
				w.mutex.Lock()
				ff.flush()
				w.mutex.Unlock()
			case <-stop:
				ticker.Stop()
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

// acquire increments the reference count for this writer.
func (w *fileWriter) acquire() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.refs++
}

// release decrements the reference count. If the writer was evicted and this
// was the last reference, it shuts down the writer.
func (w *fileWriter) release() error {
	w.mutex.Lock()
	w.refs--
	handOff := w.refs == 0 && w.evicted
	w.mutex.Unlock()
	if !handOff {
		return nil
	}
	return w.shutdown()
}

// evict marks the writer as evicted and shuts it down if there are no more references.
func (w *fileWriter) evict() error {
	w.mutex.Lock()
	w.evicted = true
	idle := w.refs == 0
	w.mutex.Unlock()
	if !idle {
		return nil
	}
	return w.shutdown()
}

// shutdown stops the flusher and closes the file. It is safe to call multiple times.
func (w *fileWriter) shutdown() error {
	w.stopFlusher()
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	// Close the file. This will flush any buffered data.
	return w.file.Close()
}

// stopFlusher stops the flusher if it is running.
func (w *fileWriter) stopFlusher() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.stopTicker == nil {
		return
	}
	close(w.stopTicker)
	w.stopTicker = nil
	w.flushTicker = nil
}

func buildExportFunc(cfg *Config) func(w *fileWriter, buf []byte) error {
	if metadata.ExporterFileNativeCompressionFeatureGate.IsEnabled() && cfg.Compression != "" {
		// Native compression: the compression stream handles framing, so
		// JSON can use newline-delimited output (human-readable after decompression).
		// Proto still needs length-prefix for message boundary detection.
		// When a custom encoding extension is set, the actual wire format may be
		// binary regardless of FormatType, so length-prefix framing is required.
		if cfg.FormatType == formatTypeJSON && cfg.Encoding == nil {
			return exportMessageAsLine
		}
		return exportMessageAsBuffer
	}
	// Legacy behavior
	if cfg.FormatType == formatTypeProto {
		return exportMessageAsBuffer
	}
	// if the data format is JSON and needs to be compressed, telemetry data can't be written to file in JSON format.
	if cfg.FormatType == formatTypeJSON && cfg.Compression != "" {
		return exportMessageAsBuffer
	}
	return exportMessageAsLine
}
