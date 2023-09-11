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

// Build builds Multiline Splitter struct
func (f *splitFuncFactory) Build() (bufio.SplitFunc, error) {
	splitFunc, err := f.splitConfig.Func(f.encoding, false, f.maxLogSize, f.trimFunc)
	if err != nil {
		return nil, err
	}
	return flush.WithPeriod(splitFunc, f.trimFunc, f.flushPeriod), nil
}
