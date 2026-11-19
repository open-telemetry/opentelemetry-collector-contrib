// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"bytes"
	"io"
)

// Nop copies input to output without compressing or decompressing it.
// It implements both Reader and Writer for configurations
// where no compression is selected.
type Nop struct{}

var (
	_ Reader = (Nop{})
	_ Writer = (Nop{})

	_ ReadLeaser  = (Nop{})
	_ WriteLeaser = (Nop{})
)

// Compress copies src to dst and returns the number of bytes copied.
func (Nop) Compress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// CompressBytes copies b to dst and returns the number of bytes written.
func (nop Nop) CompressBytes(dst io.Writer, b []byte) (int64, error) {
	return nop.fromBytes(dst, b)
}

// Decompress copies src to dst and returns the number of bytes copied.
func (Nop) Decompress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// DecompressBytes copies b to dst and returns the number of bytes written.
func (nop Nop) DecompressBytes(dst io.Writer, b []byte) (int64, error) {
	return nop.fromBytes(dst, b)
}

func (Nop) LeaseReader(leaser func(ResettableReader) error) error {
	pr := &passthroughReader{Reader: bytes.NewReader(nil)}
	return leaser(pr)
}

func (Nop) LeaseWriter(leaser func(ResettableWriter) error) error {
	pw := &passthroughWriter{Writer: bytes.NewBuffer(nil)}
	return leaser(pw)
}

func (Nop) fromBytes(dst io.Writer, b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	// Write directly to avoid allocating an intermediate reader to the heap.
	n, err := dst.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return int64(n), err
}
