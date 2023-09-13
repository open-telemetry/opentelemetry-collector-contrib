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

type splitFuncFactory struct {
	splitConfig split.Config
	encoding    encoding.Encoding
	maxLogSize  int
	trimFunc    trim.Func
	flushPeriod time.Duration
}

var _ Factory = (*splitFuncFactory)(nil)

func NewSplitFuncFactory(
	splitConfig split.Config,
	encoding encoding.Encoding,
	maxLogSize int,
	trimFunc trim.Func,
	flushPeriod time.Duration,
) Factory {
	return &splitFuncFactory{
		splitConfig: splitConfig,
		encoding:    encoding,
		maxLogSize:  maxLogSize,
		trimFunc:    trimFunc,
		flushPeriod: flushPeriod,
	}
}

// SplitFunc builds a bufio.SplitFunc based on the configuration
func (f *splitFuncFactory) SplitFunc() (bufio.SplitFunc, error) {
	splitFunc, err := f.splitConfig.Func(f.encoding, false, f.maxLogSize)
	if err != nil {
		return nil, err
	}
	splitFunc = flush.WithPeriod(splitFunc, f.flushPeriod)
	if f.encoding == encoding.Nop {
		// Special case where we should never trim
		return splitFunc, nil
	}
	return trim.WithFunc(splitFunc, f.trimFunc), nil
}
