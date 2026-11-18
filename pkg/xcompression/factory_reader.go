// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configcompression"
)

// ReaderConstructorFunc constructs a Reader from compression parameters.
type ReaderConstructorFunc func(params configcompression.CompressionParams) Reader

// New constructs a Reader using fn.
func (fn ReaderConstructorFunc) New(params configcompression.CompressionParams) Reader {
	return fn(params)
}

// ReaderFactory creates readers for supported compression types.
type ReaderFactory struct {
	constructors map[configcompression.Type]ReaderConstructorFunc
}

// NewReaderFactory returns a factory for the supported compression types.
func NewReaderFactory() *ReaderFactory {
	return &ReaderFactory{
		constructors: map[configcompression.Type]ReaderConstructorFunc{
			"": func(_ configcompression.CompressionParams) Reader {
				return Nop{}
			},
			"none": func(_ configcompression.CompressionParams) Reader {
				return Nop{}
			},
			configcompression.TypeGzip:    newGZIPCompressReaderPool,
			configcompression.TypeZlib:    newZLibCompressReaderPool,
			configcompression.TypeDeflate: newFlateCompressReaderPool,
			configcompression.TypeSnappy:  newSnappyCompressReaderPool,
			configcompression.TypeZstd:    newZSTDCompressReaderPool,
			configcompression.TypeLz4:     newLZ4CompressReaderPool,
		},
	}
}

// NewReader returns a Reader for typ, configured with params. It returns an error
// when typ is not supported.
func (rf *ReaderFactory) NewReader(typ configcompression.Type, params configcompression.CompressionParams) (Reader, error) {
	constructor, ok := rf.constructors[typ]
	if !ok {
		return nil, fmt.Errorf("no configured compression writer for %q", typ)
	}
	return constructor.New(params), nil
}

// NewReadLeaser returns a [ReadLeaser] for typ, configured with params.
// It returns an error if typ is unsupported or selects an implementation without
// leasing support, including an empty type or "none".
func (rf *ReaderFactory) NewReadLeaser(typ configcompression.Type, params configcompression.CompressionParams) (ReadLeaser, error) {
	r, err := rf.NewReader(typ, params)
	if err != nil {
		return nil, err
	}
	if leaser, ok := r.(ReadLeaser); ok {
		return leaser, nil
	}
	return nil, fmt.Errorf("compression %q does not support read leasing", typ)
}
