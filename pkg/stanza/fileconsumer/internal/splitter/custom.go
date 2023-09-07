// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type customFactory struct {
	flusherCfg tokenize.FlusherConfig
	splitFunc  bufio.SplitFunc
}

var _ Factory = (*customFactory)(nil)

func NewCustomFactory(flusherCfg tokenize.FlusherConfig, splitFunc bufio.SplitFunc) Factory {
	return &customFactory{
		flusherCfg: flusherCfg,
		splitFunc:  splitFunc,
	}
}

// Build builds Multiline Splitter struct
func (f *customFactory) Build() (bufio.SplitFunc, error) {
	return f.flusherCfg.Wrap(f.splitFunc, trim.Nop), nil
}
