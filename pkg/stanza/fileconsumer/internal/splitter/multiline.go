// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"
	"time"

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type multilineFactory struct {
	multilineCfg split.MultilineConfig
	encoding     encoding.Encoding
	maxLogSize   int
	trimFunc     trim.Func
	flushPeriod  time.Duration
}

var _ Factory = (*multilineFactory)(nil)

func NewMultilineFactory(
	multilineCfg split.MultilineConfig,
	encoding encoding.Encoding,
	maxLogSize int,
	trimFunc trim.Func,
	flushPeriod time.Duration,
) Factory {
	return &multilineFactory{
		multilineCfg: multilineCfg,
		encoding:     encoding,
		maxLogSize:   maxLogSize,
		trimFunc:     trimFunc,
		flushPeriod:  flushPeriod,
	}
}

// Build builds Multiline Splitter struct
func (f *multilineFactory) Build() (bufio.SplitFunc, error) {
	splitFunc, err := f.multilineCfg.Build(f.encoding, false, f.maxLogSize, f.trimFunc)
	if err != nil {
		return nil, err
	}
	return flush.WithPeriod(splitFunc, f.trimFunc, f.flushPeriod), nil
}
