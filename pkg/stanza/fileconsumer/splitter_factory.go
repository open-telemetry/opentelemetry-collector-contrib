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
	Build(maxLogSize int) (*helper.Splitter, error)
}

type defaultSplitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
}

func (factory *defaultSplitterFactory) Build(maxLogSize int) (*helper.Splitter, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitFunc := helper.SplitNone(maxLogSize)
	if flusher != nil {
		splitFunc = flusher.SplitFunc(splitFunc)
	}
	return &helper.Splitter{
		Encoding:  enc,
		Flusher:   flusher,
		SplitFunc: splitFunc,
	}, nil
}

type multilineSplitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
	Multiline      helper.MultilineConfig
}

func (factory *multilineSplitterFactory) Build(maxLogSize int) (*helper.Splitter, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitter, err := factory.Multiline.Build(enc.Encoding, false, flusher, maxLogSize)
	if err != nil {
		return nil, err
	}
	if flusher != nil {
		splitter = flusher.SplitFunc(splitter)
	}
	return &helper.Splitter{
		Encoding:  enc,
		Flusher:   flusher,
		SplitFunc: splitter,
	}, nil
}

type customizeSplitterFactory struct {
	EncodingConfig helper.EncodingConfig
	Flusher        helper.FlusherConfig
	SplitFunc      bufio.SplitFunc
}

func (factory *customizeSplitterFactory) Build(maxLogSize int) (*helper.Splitter, error) {
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitFunc := factory.SplitFunc
	if flusher != nil {
		splitFunc = flusher.SplitFunc(splitFunc)
	}
	return &helper.Splitter{
		Encoding:  enc,
		Flusher:   flusher,
		SplitFunc: splitFunc,
	}, nil
}
