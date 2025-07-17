// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pool // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/pool"

import (
	"bytes"
	"io"
	"sync"
)

type BufferPool struct {
	pool *sync.Pool
}

func NewBufferPool() *BufferPool {
	return &BufferPool{pool: &sync.Pool{New: func() any { return &bytes.Buffer{} }}}
}

func (w *BufferPool) NewPooledBuffer() PooledBuffer {
	return PooledBuffer{
		Buffer: w.pool.Get().(*bytes.Buffer),
		pool:   w.pool,
	}
}

type PooledBuffer struct {
	Buffer *bytes.Buffer
	pool   *sync.Pool
}

func (p PooledBuffer) Recycle() {
	p.Buffer.Reset()
	p.pool.Put(p.Buffer)
}

func (p PooledBuffer) WriteTo(w io.Writer) (n int64, err error) {
	defer p.Recycle()
	return p.Buffer.WriteTo(w)
}
