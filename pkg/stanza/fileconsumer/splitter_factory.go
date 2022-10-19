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

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type splitterFactory interface {
	Build(maxLogSize int) (bufio.SplitFunc, error)
}

func NewFactory(opts ...FactoryOption) splitterFactory {
	builder := &splitterBuilder{}
	for _, opt := range opts {
		opt(builder)
	}
	if builder.multiline.LineStartPattern != "" || builder.multiline.LineEndPattern != "" {
		return newMultilineSplitterFactory(builder.encoding, builder.flusher, builder.multiline)
	}
	return newDefaultSplitterFactory(builder.encoding, builder.flusher)
}

type multilineSplitterFactory struct {
	Encoding  encoding.Encoding
	Flusher   *helper.Flusher
	Multiline helper.MultilineConfig
}

var _ splitterFactory = (*multilineSplitterFactory)(nil)

func newMultilineSplitterFactory(
	encoding encoding.Encoding,
	flusher *helper.Flusher,
	multiline helper.MultilineConfig) *multilineSplitterFactory {
	return &multilineSplitterFactory{
		Encoding:  encoding,
		Flusher:   flusher,
		Multiline: multiline,
	}

}

// Build builds Multiline Splitter struct
func (factory *multilineSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	splitter, err := factory.Multiline.Build(factory.Encoding, false, factory.Flusher, maxLogSize)
	if err != nil {
		return nil, err
	}
	return splitter, nil
}

type defaultSplitterFactory struct {
	Encoding encoding.Encoding
	Flusher  *helper.Flusher
}

var _ splitterFactory = (*defaultSplitterFactory)(nil)

func newDefaultSplitterFactory(
	encoding encoding.Encoding,
	flusher *helper.Flusher) *defaultSplitterFactory {
	return &defaultSplitterFactory{
		Encoding: encoding,
		Flusher:  flusher,
	}
}

// Build builds default Splitter struct
func (factory *defaultSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	if factory.Encoding == encoding.Nop {
		return helper.SplitNone(maxLogSize), nil
	}
	splitFunc, err := helper.NewNewlineSplitFunc(factory.Encoding, false)
	if err != nil {
		return nil, err
	}
	if factory.Flusher != nil {
		splitFunc = factory.Flusher.SplitFunc(splitFunc)
	}
	return splitFunc, nil
}

type splitterBuilder struct {
	flusher   *helper.Flusher
	encoding  encoding.Encoding
	multiline helper.MultilineConfig
}

type FactoryOption func(*splitterBuilder)

func WithFlusher(flusher *helper.Flusher) func(*splitterBuilder) {
	return func(builder *splitterBuilder) {
		builder.flusher = flusher
	}
}

func WithMultiline(multiline helper.MultilineConfig) func(*splitterBuilder) {
	return func(builder *splitterBuilder) {
		builder.multiline = multiline
	}
}

func WithEncoding(encoding encoding.Encoding) func(*splitterBuilder) {
	return func(builder *splitterBuilder) {
		builder.encoding = encoding
	}
}
