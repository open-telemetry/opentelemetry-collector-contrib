// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"bytes"
	"encoding/json"
	"sync"
)

const (
	maxBufSize = 65536
)

type writer struct {
	buffer  *bytes.Buffer
	encoder *json.Encoder
}

type writerPool struct {
	pool *sync.Pool
}

func newWriterPool(size int) *writerPool {
	pool := &sync.Pool{
		New: func() interface{} {
			var (
				buffer  = bytes.NewBuffer(make([]byte, 0, size))
				encoder = json.NewEncoder(buffer)
			)

			return &writer{
				buffer:  buffer,
				encoder: encoder,
			}
		},
	}
	return &writerPool{pool: pool}
}

func (w *writer) Reset() {
	w.buffer.Reset()
}

func (w *writer) Encode(v interface{}) error {
	return w.encoder.Encode(v)
}

func (w *writer) String() string {
	return w.buffer.String()
}

func (writerPool *writerPool) borrow() *writer {
	return writerPool.pool.Get().(*writer)
}

func (writerPool *writerPool) release(w *writer) {
	if w.buffer.Cap() < maxBufSize {
		w.buffer.Reset()
		writerPool.pool.Put(w)
	}
}
