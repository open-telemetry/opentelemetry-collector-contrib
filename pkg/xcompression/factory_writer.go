// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configcompression"
)

// WriterConstructionFunc constructs a Writer from compression parameters.
type WriterConstructionFunc func(params configcompression.CompressionParams) (Writer, error)

// New constructs a Writer using fn.
func (fn WriterConstructionFunc) New(params configcompression.CompressionParams) (Writer, error) {
	return fn(params)
}

// WriterFactory creates writers for supported compression types.
type WriterFactory struct {
	constructors map[configcompression.Type]WriterConstructionFunc
}

// NewWriterFactory returns a factory for the supported compression types.
func NewWriterFactory() *WriterFactory {
	return &WriterFactory{
		constructors: map[configcompression.Type]WriterConstructionFunc{
			"": func(_ configcompression.CompressionParams) (Writer, error) {
				return Nop{}, nil
			},
			"none": func(_ configcompression.CompressionParams) (Writer, error) {
				return Nop{}, nil
			},
			configcompression.TypeGzip:         newGZIPCompressWriterPool,
			configcompression.TypeZlib:         newZLibCompressWriterPool,
			configcompression.TypeDeflate:      newFlateCompressWriterPool,
			configcompression.TypeSnappy:       newSnappyCompressWriterPool,
			configcompression.TypeSnappyFramed: newSnappyCompressWriterPool,
			configcompression.TypeZstd:         newZSTDCompressWriterPool,
			configcompression.TypeLz4:          newLZ4CompressWriterPool,
		},
	}
}

// Get returns a Writer for typ, configured with params. It returns an error
// when typ is not supported or the writer cannot be constructed.
func (cwf *WriterFactory) Get(typ configcompression.Type, params configcompression.CompressionParams) (Writer, error) {
	constructor, ok := cwf.constructors[typ]
	if !ok {
		return nil, fmt.Errorf("no configured compression writer for %q", typ)
	}
	return constructor.New(params)
}
