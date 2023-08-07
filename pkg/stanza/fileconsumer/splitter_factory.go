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
)

type splitterFactory interface {
	Build(maxLogSize int) (bufio.SplitFunc, error)
}

type multilineSplitterFactory struct {
	helper.SplitterConfig
}

var _ splitterFactory = (*multilineSplitterFactory)(nil)

func newMultilineSplitterFactory(splitter helper.SplitterConfig) *multilineSplitterFactory {
	return &multilineSplitterFactory{
		SplitterConfig: splitter,
	}
}

// Build builds Multiline Splitter struct
func (factory *multilineSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	enc, err := helper.LookupEncoding(factory.EncodingConfig.Encoding)
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitter, err := factory.Multiline.Build(enc, false, factory.PreserveLeadingWhitespaces, factory.PreserveTrailingWhitespaces, flusher, maxLogSize)
	if err != nil {
		return nil, err
	}
	return splitter, nil
}

type customizeSplitterFactory struct {
	Flusher  helper.FlusherConfig
	Splitter bufio.SplitFunc
}

var _ splitterFactory = (*customizeSplitterFactory)(nil)

func newCustomizeSplitterFactory(
	flusher helper.FlusherConfig,
	splitter bufio.SplitFunc) *customizeSplitterFactory {
	return &customizeSplitterFactory{
		Flusher:  flusher,
		Splitter: splitter,
	}
}

// Build builds Multiline Splitter struct
func (factory *customizeSplitterFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	flusher := factory.Flusher.Build()
	if flusher != nil {
		return flusher.SplitFunc(factory.Splitter), nil
	}
	return factory.Splitter, nil
}
