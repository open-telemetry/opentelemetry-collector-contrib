// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decoder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type multilineFactory struct {
	helper.SplitterConfig
}

var _ Factory = (*multilineFactory)(nil)

func NewMultilineFactory(splitter helper.SplitterConfig) Factory {
	return &multilineFactory{
		SplitterConfig: splitter,
	}
}

// Build builds Multiline Splitter struct
func (factory *multilineFactory) Build(maxLogSize int) (bufio.SplitFunc, error) {
	enc, err := decoder.LookupEncoding(factory.Encoding)
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
