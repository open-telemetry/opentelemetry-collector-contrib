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
	Build(maxLogSize int, enc encoding.Encoding, force *helper.Flusher) (bufio.SplitFunc, error)
}

func NewFactory(opts ...FactoryOption) splitterFactory {
	builder := &splitterBuilder{}
	for _, opt := range opts {
		opt(builder)
	}
	if builder.multiline.LineStartPattern != "" || builder.multiline.LineEndPattern != "" {
		return newMultilineSplitterFactory(builder.multiline)
	}
	return newDefaultSplitterFactory()
}

type multilineSplitterFactory struct {
	Multiline helper.MultilineConfig
}

var _ splitterFactory = (*multilineSplitterFactory)(nil)

func newMultilineSplitterFactory(
	multiline helper.MultilineConfig) *multilineSplitterFactory {
	return &multilineSplitterFactory{
		Multiline: multiline,
	}

}

// Build builds Multiline Splitter struct
func (factory *multilineSplitterFactory) Build(maxLogSize int, enc encoding.Encoding, force *helper.Flusher) (bufio.SplitFunc, error) {
	splitter, err := factory.Multiline.Build(enc, false, force, maxLogSize)
	if err != nil {
		return nil, err
	}
	return splitter, nil
}

type defaultSplitterFactory struct {
}

var _ splitterFactory = (*defaultSplitterFactory)(nil)

func newDefaultSplitterFactory() *defaultSplitterFactory {
	return &defaultSplitterFactory{}
}

// Build builds default Splitter struct
func (factory *defaultSplitterFactory) Build(maxLogSize int, enc encoding.Encoding, force *helper.Flusher) (bufio.SplitFunc, error) {
	if enc == encoding.Nop {
		return helper.SplitNone(maxLogSize), nil
	}
	splitFunc, err := helper.NewNewlineSplitFunc(enc, false)
	if err != nil {
		return nil, err
	}
	if force != nil {
		splitFunc = force.SplitFunc(splitFunc)
	}
	return splitFunc, nil
}

type splitterBuilder struct {
	multiline helper.MultilineConfig
}

type FactoryOption func(*splitterBuilder)

func WithMultiline(multiline helper.MultilineConfig) func(*splitterBuilder) {
	return func(builder *splitterBuilder) {
		builder.multiline = multiline
	}
}
