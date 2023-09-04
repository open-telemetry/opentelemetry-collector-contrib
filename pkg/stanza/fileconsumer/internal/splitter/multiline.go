// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"
)

type multilineFactory struct {
	splitterCfg tokenize.SplitterConfig
	maxLogSize  int
}

var _ Factory = (*multilineFactory)(nil)

func NewMultilineFactory(splitterCfg tokenize.SplitterConfig, maxLogSize int) Factory {
	return &multilineFactory{splitterCfg: splitterCfg, maxLogSize: maxLogSize}
}

// Build builds Multiline Splitter struct
func (f *multilineFactory) Build() (bufio.SplitFunc, error) {
	return f.splitterCfg.Build(false, f.maxLogSize)
}
