// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	enc, err := factory.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := factory.Flusher.Build()
	splitter, err := factory.Multiline.Build(enc.Encoding, false, factory.PreserveLeadingWhitespaces, factory.PreserveTrailingWhitespaces, flusher, maxLogSize)
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
