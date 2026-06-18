// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// defaultMaxFrameMegabytes mirrors timberjack's default MaxSize.
const defaultMaxFrameMegabytes = 100

// compressingWriter wraps an io.WriteCloser with zstd compression.
//
//   - rotation != nil: each Write() is compressed into one complete frame (via
//     EncodeAll) and written atomically. timberjack rotates between Write calls
//     but never splits one, so a streamed frame (header/blocks/CRC across several
//     writes) could be split across files; writing whole frames keeps every
//     rotated file a valid, zstd -d-decodable .zst.
//   - rotation == nil: a single stream stays open, finalized by flush()/Close()
//     for a better ratio.
//
// Not thread-safe; callers serialize via fileWriter.mutex.
type compressingWriter struct {
	base          io.WriteCloser // underlying writer (file or timberjack)
	compression   string
	level         int
	encoder       *zstd.Encoder
	rotation      *Rotation // when non-nil, finalize a frame per Write()
	maxFrameBytes int       // rotation mode: max bytes for a single frame
	frame         []byte    // rotation mode: reusable EncodeAll output buffer
	dirty         bool      // encoder has received data since last flush/creation
	err           error     // sticky error state
}

func newCompressingWriter(base io.WriteCloser, compression string, level int, rotation *Rotation) (*compressingWriter, error) {
	cw := &compressingWriter{
		base:        base,
		compression: compression,
		level:       level,
		rotation:    rotation,
	}

	// Rotation mode uses EncodeAll only, so the encoder needs no streaming target.
	var target io.Writer
	if rotation == nil {
		target = base
	} else {
		maxMB := rotation.MaxMegabytes
		if maxMB <= 0 {
			maxMB = defaultMaxFrameMegabytes
		}
		cw.maxFrameBytes = maxMB * 1024 * 1024
	}

	encoder, err := cw.newEncoder(target)
	if err != nil {
		return nil, err
	}
	cw.encoder = encoder

	return cw, nil
}

func (c *compressingWriter) newEncoder(w io.Writer) (*zstd.Encoder, error) {
	switch c.compression {
	case compressionZSTD:
		return zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(c.level)),
			zstd.WithEncoderConcurrency(1),
		)
	default:
		return nil, fmt.Errorf("unsupported compression: %s", c.compression)
	}
}

func (c *compressingWriter) Write(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}

	// Non-rotation: stream directly; flush()/Close() finalize the frame.
	if c.rotation == nil {
		n, err := c.encoder.Write(p)
		if err != nil {
			c.err = err
			return n, err
		}
		c.dirty = true
		return n, nil
	}

	return c.writeFrame(p)
}

// writeFrame compresses p into one complete frame and writes it in a single
// Write so a rotation cannot split it. A frame larger than the rotation limit
// (which the writer would reject) is split into in-bounds chunks; the
// decompressed frames concatenate back to p.
func (c *compressingWriter) writeFrame(p []byte) (int, error) {
	c.frame = c.encoder.EncodeAll(p, c.frame[:0])
	if c.maxFrameBytes > 0 && len(c.frame) > c.maxFrameBytes {
		return c.writeChunkedFrames(p)
	}
	if _, err := c.base.Write(c.frame); err != nil {
		c.err = err
		return 0, err
	}
	return len(p), nil
}

// writeChunkedFrames splits an oversized record into chunks that each compress
// below maxFrameBytes (with headroom for zstd overhead so even incompressible
// data fits) and writes each as its own frame.
func (c *compressingWriter) writeChunkedFrames(p []byte) (int, error) {
	chunkSize := c.maxFrameBytes - c.maxFrameBytes/100 - 4096
	if chunkSize < 1 {
		chunkSize = c.maxFrameBytes
	}

	written := 0
	for len(p) > 0 {
		n := min(chunkSize, len(p))
		c.frame = c.encoder.EncodeAll(p[:n], c.frame[:0])
		if _, err := c.base.Write(c.frame); err != nil {
			c.err = err
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

// closeAndResetEncoder finalizes the current frame and resets the encoder.
// Non-rotation only.
func (c *compressingWriter) closeAndResetEncoder() error {
	if err := c.encoder.Close(); err != nil {
		return err
	}
	c.encoder.Reset(c.base)
	c.dirty = false
	return nil
}

// Close finalizes the compression stream and closes the underlying writer.
func (c *compressingWriter) Close() error {
	// Non-rotation: Close() finalizes the open frame into base. Rotation: the
	// encoder has no stream (EncodeAll only), so this just releases resources.
	encoderErr := c.encoder.Close()
	baseErr := c.base.Close()
	return errors.Join(encoderErr, baseErr)
}

// flush finalizes the current frame if dirty. In rotation mode dirty is never
// set, so this is a no-op.
func (c *compressingWriter) flush() error {
	if !c.dirty {
		return nil
	}
	return c.closeAndResetEncoder()
}
