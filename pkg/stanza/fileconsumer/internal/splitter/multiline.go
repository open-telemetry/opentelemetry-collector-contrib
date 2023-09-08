// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type multilineFactory struct {
	splitterCfg tokenize.SplitterConfig
	encoding    encoding.Encoding
	maxLogSize  int
	trimFunc    trim.Func
}

var _ Factory = (*multilineFactory)(nil)

func NewMultilineFactory(
	splitterCfg tokenize.SplitterConfig,
	encoding encoding.Encoding,
	maxLogSize int,
	trimFunc trim.Func,
) Factory {
	return &multilineFactory{
		splitterCfg: splitterCfg,
		encoding:    encoding,
		maxLogSize:  maxLogSize,
		trimFunc:    trimFunc,
	}
}

// Build builds Multiline Splitter struct
func (f *multilineFactory) Build() (bufio.SplitFunc, error) {
	return f.splitterCfg.Build(f.encoding, false, f.maxLogSize, f.trimFunc)
}
