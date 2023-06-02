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

package compress // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"

import "io"

type noop struct {
	data io.Writer
}

func NewNoopCompressor() Compressor {
	return &compressor{
		compression: &noop{},
	}
}

func (n *noop) Reset(w io.Writer) {
	n.data = w
}

func (n noop) Write(p []byte) (int, error) {
	return n.data.Write(p)
}

func (n noop) Flush() error {
	return nil
}
