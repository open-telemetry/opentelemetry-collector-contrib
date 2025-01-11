// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"io"
	"sync"
)

type bufferPool struct {
	pool *sync.Pool
}

func newBufferPool() *bufferPool {
	return &bufferPool{pool: &sync.Pool{New: func() any { return &bytes.Buffer{} }}}
}

func (w *bufferPool) newPooledBuffer() pooledBuffer {
	return pooledBuffer{
		Buffer: w.pool.Get().(*bytes.Buffer),
		pool:   w.pool,
	}
}

type pooledBuffer struct {
	Buffer *bytes.Buffer
	pool   *sync.Pool
}

func (p pooledBuffer) recycle() {
	p.Buffer.Reset()
	p.pool.Put(p.Buffer)
}

func (p pooledBuffer) WriteTo(w io.Writer) (n int64, err error) {
	defer p.recycle()
	return p.Buffer.WriteTo(w)
}
