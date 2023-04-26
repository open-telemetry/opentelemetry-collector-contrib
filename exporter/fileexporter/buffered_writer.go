// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"bufio"
	"io"

	"go.uber.org/multierr"
)

// bufferedWriteCloser is intended to use more memory
// in order to optimize writing to disk to help improve performance.
type bufferedWriteCloser struct {
	wrapped  io.Closer
	buffered *bufio.Writer
}

var (
	_ io.WriteCloser = (*bufferedWriteCloser)(nil)
)

func newBufferedWriteCloser(f io.WriteCloser) io.WriteCloser {
	return &bufferedWriteCloser{
		wrapped:  f,
		buffered: bufio.NewWriter(f),
	}
}

func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
	return bwc.buffered.Write(p)
}

func (bwc *bufferedWriteCloser) Close() error {
	return multierr.Combine(
		bwc.buffered.Flush(),
		bwc.wrapped.Close(),
	)
}

func (bwc *bufferedWriteCloser) flush() error {
	return bwc.buffered.Flush()
}
