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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"golang.org/x/text/encoding"
)

type splitterFactory interface {
	Build(maxLogSize int) (bufio.SplitFunc, error)
}

type multilineSplitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
	Multiline      helper.MultilineConfig
}

var _ splitterFactory = (*multilineSplitterFactory)(nil)

func newMultilineSplitterFactory(
	encoding helper.EncodingConfig,
	flusher helper.FlusherConfig,
	multiline helper.MultilineConfig) *multilineSplitterFactory {
	return &multilineSplitterFactory{
		EncodingConfig: encoding,
		Flusher:        flusher,
		Multiline:      multiline,
	}

}

// Build builds Multiline Splitter struct
func (factory *multilineSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitter, err := factory.Multiline.Build(enc.Encoding, false, flusher, maxLogSize)
	if err != nil {
		return nil, err
	}
	return splitter, nil
}

type defaultSplitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
}

var _ splitterFactory = (*defaultSplitterFactory)(nil)

func newDefaultSplitterFactory(
	encoding helper.EncodingConfig,
	flusher helper.FlusherConfig) *defaultSplitterFactory {
	return &defaultSplitterFactory{
		EncodingConfig: encoding,
		Flusher:        flusher,
	}
}

// Build builds default Splitter struct
func (factory *defaultSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	if enc.Encoding == encoding.Nop {
		return helper.SplitNone(maxLogSize), nil
	}
	splitFunc, err := helper.NewNewlineSplitFunc(enc.Encoding, false)
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	if flusher != nil {
		splitFunc = flusher.SplitFunc(splitFunc)
	}
	return splitFunc, nil
}
